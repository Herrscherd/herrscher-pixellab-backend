# herrscher-pixellab-backend

A Herrscher backend that generates pixel art with [PixelLab](https://www.pixellab.ai) instead of a language model. A prompt goes in, PNG files that have passed a check come out in the outbox, and the reply names them.

Category: backend. Status: live.

## Install

```sh
go get github.com/Herrscherd/herrscher-pixellab-backend
```

The module registers itself with `contracts.Register` in its `init`, under the kind `pixellab`. Herrscher picks it up by importing the module in its plugin list.

## Herrscher configuration

Select the backend by its kind:

```yaml
backend: pixellab
```

Then set the settings below, either in the herrscher configuration or through the environment variables.

## Settings

| Key | Environment variable | Default | Description |
|---|---|---|---|
| `accounts` | `PIXELLAB_ACCOUNTS` | none | Inline account list, in the [account format](#account-format). |
| `accounts_file` | `PIXELLAB_ACCOUNTS_FILE` | none | Path to a file holding the same account list. Read when `accounts` is empty. |
| `aseprite_dir` | `PIXELLAB_ASEPRITE_DIR` | `~/.config/aseprite/extensions` | Directory of the Aseprite extensions. Its `pixellab/package.json` supplies one account when no other source is set. |
| `endpoint` | `PIXELLAB_ENDPOINT` | `wss://api.pixellab.ai` | Base WebSocket URL. `wss://` for any host; `ws://` only for a loopback address (`localhost`, `127.0.0.1`, `::1`). |
| `default_size` | `PIXELLAB_DEFAULT_SIZE` | `64` | Side in pixels of a generated image when the prompt has no `--size`. Maximum 256. |
| `max_images` | `PIXELLAB_MAX_IMAGES` | `4` | Largest value accepted for `--n` in one prompt. |
| `timeout` | `PIXELLAB_TIMEOUT` | `10m` | Time limit of one request, a Go duration such as `90s` or `12m`. |
| `turn_timeout` | `PIXELLAB_TURN_TIMEOUT` | `30m` | Time limit of one whole turn, every account and image included, a Go duration. |

At least one account source must resolve to an account. Accounts are read in this order: `accounts` or `PIXELLAB_ACCOUNTS`, then `accounts_file`, then the Aseprite extension.

## Account format

Each account is one secret, optionally followed by the client version and the tier:

```text
<secret> [| version] [| tier]
```

- Accounts are separated by `;` or by a newline.
- `#` starts a comment that runs to the end of the line.
- `version` is the PixelLab client version. A stale version fails on the server, so write it explicitly when the Aseprite extension is not installed.
- `tier` is a positive number. It defaults to the tier of the Aseprite extension, or to `1`.

Example:

```text
# main subscription
secret-main | 0.4.120 | 2
secret-spare | 0.4.120
```

Accounts are tried in order. The secrets are never written to replies, and errors show them redacted.

## Prompt options

A prompt is a free description, optionally followed by `--flag value` options. Quote the description when it contains spaces that matter to the parser, for example `"a knight with a red cape"`.

| Option | Meaning |
|---|---|
| `--size N` | Side in pixels, up to 256. Defaults to `default_size`. |
| `--n N` | Number of images. Each image is its own request with seed + index. Up to `max_images`. |
| `--seed N` | Seed of the first image. Without it, a random seed is drawn. |
| `--grid N` | Grid resolution of the generator, up to 8. Defaults from the size. |
| `--bg` | Keep the background. By default the background is removed. |
| `--neg TEXT` | Negative description, what the image must not contain. |
| `--mode MODE` | `new` (default), `edit`, `inpaint` or `animate`. |
| `--action TEXT` | Animation action, used with `--mode animate`. |
| `--frames N` | Number of animation frames, up to 16. Defaults to 8. |
| `--palette` | Enable the palette style. |
| `--no-outline` | Turn off the outline style. |
| `--no-detail` | Turn off the detail style. |
| `--no-shading` | Turn off the shading style. |
| `--crop` | Crop the result to the mask, with `inpaint`. |

Example prompts:

```text
a copper lantern on a wooden table --size 128 --n 3 --seed 42
"a slime with a crown" --size 64 --neg blur --no-shading
```

Each frame that PixelLab returns is checked, then saved as `pixellab-NN.png` in the outbox, and the reply lists the files. A frame is a PNG or raw RGBA with sides from 1 to 256 px. A PNG that does not decode, a frame over 8 MiB, or a request that receives more than 10 000 frames before the final image fails the turn with an error. The frame that failed is not written; the files of the turn already written stay in the outbox.

## Modes and attachments

Attachments must be PNG files, square, and of the size given by `--size` when that option is present. Their count depends on the mode.

| Mode | Attachments | Use |
|---|---|---|
| `new` | none, an attachment is rejected | Text to image. |
| `edit` | 1 image | Edit the attached image from the description. |
| `inpaint` | 2 images: the source, then the mask | Repaint the area the mask marks. |
| `animate` | 1 or 2 images: the first frame, then optionally the last frame | Animate from `--action` over `--frames` frames. Takes no description: a description is refused. |

## Slow queue and timeout

When a subscription runs out of quota, PixelLab does not refuse the request. It puts it in a low-priority queue and sends `slow` progress messages. With several accounts, the backend marks an account that reports `slow` as dry and moves to the next one. When every account is dry, the backend rides the slow queue on the first live account. That wait takes about 5 to 7 minutes, which is why the default `timeout` is 10 minutes. Lower it only if you accept that slow jobs will fail.

A turn runs one request after another, so `turn_timeout` (default `30m`) bounds the whole turn: every account tried and every image produced. Turns run one at a time per backend; a turn waiting for the previous one stops when its context ends.

## Files and the host

The backend needs a herrscher host that provides the outbox of outgoing files: a prompt without an outbox is refused. That outbox is a feature of herrscher PR #195, not released yet, so this backend cannot run against a released herrscher until it ships. The backend writes each PNG into the prompt's outbox and reports it through `TakeFiles`. The host calls `TakeFiles` right after each `Respond`, in the same turn and before any other `Respond`. Files of a `Respond` that were not taken by then are handed over with the files of the next turn.

## Probe

The health probe only reads the configuration. It checks that the settings parse and that an account can be resolved. It makes no call to PixelLab, so it costs no quota and proves nothing about the subscription.

## Development

Run the same checks as CI from the repository root:

```sh
GOWORK=off scripts/check.sh
```

The G117 exclusion of `golangci-lint` covers `client.go` alone. The request body carries the account secret because the PixelLab protocol sends it there, so the finding is expected on that file; the exclusion matches the G117 text only, and no other file or linter is excluded.

The live test in `live_test.go` calls the real API. It runs only when `PIXELLAB_LIVE=1` is set, and it spends quota.

## License

MIT, see [LICENSE](LICENSE).

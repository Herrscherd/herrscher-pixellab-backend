# herrscher-pixellab-backend

A Herrscher backend that generates pixel art with [PixelLab](https://www.pixellab.ai) instead of a language model. A prompt goes in, PNG files come out in the outbox, and the reply names them.

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

Each generated frame is saved as `pixellab-NN.png` in the outbox, and the reply lists the files.

## Modes and attachments

Attachments must be PNG files, square, and of the size given by `--size` when that option is present. Their count depends on the mode.

| Mode | Attachments | Use |
|---|---|---|
| `new` | none, an attachment is rejected | Text to image. |
| `edit` | 1 image | Edit the attached image from the description. |
| `inpaint` | 2 images: the source, then the mask | Repaint the area the mask marks. |
| `animate` | 1 or 2 images: the first frame, then optionally the last frame | Animate from `--action` over `--frames` frames. |

## Slow queue and timeout

When a subscription runs out of quota, PixelLab does not refuse the request. It puts it in a low-priority queue and sends `slow` progress messages. With several accounts, the backend marks an account that reports `slow` as dry and moves to the next one. When every account is dry, the backend rides the slow queue on the first live account. That wait takes about 5 to 7 minutes, which is why the default `timeout` is 10 minutes. Lower it only if you accept that slow jobs will fail.

## Probe

The health probe only reads the configuration. It checks that the settings parse and that an account can be resolved. It makes no call to PixelLab, so it costs no quota and proves nothing about the subscription.

## Development

Run the same checks as CI from the repository root:

```sh
GOWORK=off scripts/check.sh
```

The live test in `live_test.go` calls the real API. It runs only when `PIXELLAB_LIVE=1` is set, and it spends quota.

## License

MIT, see [LICENSE](LICENSE).

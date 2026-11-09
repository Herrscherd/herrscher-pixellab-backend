package pixellab

import (
	"context"
	"errors"
	"os"

	contracts "github.com/Herrscherd/herrscher-contracts"
)

const (
	reasonInvalidSetting = "invalid setting"
	reasonInvalidAccount = "invalid account"
	reasonUnreadable     = "unreadable account"
)

type prober struct{}

func (prober) probe(_ context.Context, access contracts.ProbeAccess) contracts.ProbeReport {
	config, err := parseSettings(access.Config)
	if err != nil {
		return unreachable(reasonInvalidSetting)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	_, err = loadAccounts(config, home)
	switch {
	case errors.Is(err, ErrNoAccount):
		return contracts.ProbeReport{
			State:  contracts.HealthDown,
			Detail: contracts.Notice{Code: contracts.CodeHealthEnvMissing, Args: []contracts.Arg{contracts.Text("Name", "PIXELLAB_ACCOUNTS")}},
			Remedy: contracts.Notice{Code: contracts.CodeHealthRemedyEnvRestart, Args: []contracts.Arg{contracts.Text("Name", "PIXELLAB_ACCOUNTS")}},
		}
	case errors.Is(err, ErrInvalidAccount):
		return unreachable(reasonInvalidAccount)
	case err != nil:
		return unreachable(reasonUnreadable)
	}
	return contracts.ProbeReport{State: contracts.HealthOK, Detail: contracts.Notice{Code: contracts.CodeHealthConnected}}
}

func unreachable(reason string) contracts.ProbeReport {
	return contracts.ProbeReport{
		State:  contracts.HealthDown,
		Detail: contracts.Notice{Code: contracts.CodeHealthUnreachable, Args: []contracts.Arg{contracts.Text("Reason", reason)}},
	}
}

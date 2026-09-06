package main

import (
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/version"
	hubtypes "github.com/sentinel-official/hub/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/trinitystake/dvpnd/cmd"
	v2ray "github.com/trinitystake/dvpnd/services/v2ray/cli"
	wireguard "github.com/trinitystake/dvpnd/services/wireguard/cli"
	"github.com/trinitystake/dvpnd/types"
)

func main() {
	hubtypes.GetConfig().Seal()
	root := &cobra.Command{
		Use:          "dvpnd",
		SilenceUsage: true,
	}

	root.AddCommand(
		cmd.ConfigCmd(),
		cmd.KeysCmd(),
		v2ray.Command(),
		wireguard.Command(),
		cmd.StartCmd(),
		version.NewVersionCommand(),
	)

	root.PersistentFlags().String(flags.FlagHome, types.DefaultHomeDirectory, "home directory")
	root.PersistentFlags().String(flags.FlagLogFormat, "plain", "log format")
	root.PersistentFlags().String(flags.FlagLogLevel, "info", "log level")

	_ = viper.BindPFlag(flags.FlagHome, root.PersistentFlags().Lookup(flags.FlagHome))
	_ = viper.BindPFlag(flags.FlagLogFormat, root.PersistentFlags().Lookup(flags.FlagLogFormat))
	_ = viper.BindPFlag(flags.FlagLogLevel, root.PersistentFlags().Lookup(flags.FlagLogLevel))

	_ = root.Execute()
}

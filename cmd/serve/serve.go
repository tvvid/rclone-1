package serve

import (
	"errors"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/serve/ftp"
	"github.com/ncw/rclone/cmd/serve/http"
	"github.com/ncw/rclone/cmd/serve/restic"
	"github.com/ncw/rclone/cmd/serve/webdav"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(http.Command)
	Command.AddCommand(webdav.Command)
	Command.AddCommand(restic.Command)
	if ftp.Command != nil {
		Command.AddCommand(ftp.Command)
	}
	cmd.Root.AddCommand(Command)
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "serve <protocol> [opts] <remote>",
	Short: `Serve a remote over a protocol.`,
	Long: `rclone serve is used to serve a remote over a given protocol. This
command requires the use of a subcommand to specify the protocol, eg

    rclone serve http remote:

Each subcommand has its own options which you can see in their help.
`,
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("serve requires a protocol, eg 'rclone serve http remote:'")
		}
		return errors.New("unknown protocol")
	},
}

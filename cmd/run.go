package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/AlexSSD7/linsk/vm"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use: "run",
	// TODO: Fill this
	// Short: "",
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		vmMountDevName := args[1]
		fsType := args[2]

		ftpPassivePortCount := uint16(9)

		networkSharePort, err := getClosestAvailPortWithSubsequent(9000, 10)
		if err != nil {
			slog.Error("Failed to get closest available host port for network file share", "error", err)
			os.Exit(1)
		}

		ports := []vm.PortForwardingRule{{
			HostIP:   net.ParseIP("127.0.0.1"), // TODO: Make this changeable.
			HostPort: networkSharePort,
			VMPort:   21,
		}}

		for i := uint16(0); i < ftpPassivePortCount; i++ {
			p := networkSharePort + 1 + i
			ports = append(ports, vm.PortForwardingRule{
				HostIP:   net.ParseIP("127.0.0.1"), // TODO: Make this changeable.
				HostPort: p,
				VMPort:   p,
			})
		}

		// TODO: `slog` library prints entire stack traces for errors which makes reading errors challenging.

		os.Exit(runVM(args[0], func(ctx context.Context, i *vm.VM, fm *vm.FileManager) int {
			err := fm.Mount(vmMountDevName, vm.MountOptions{
				FSType: fsType,
				LUKS:   luksFlag,
			})
			if err != nil {
				slog.Error("Failed to mount the disk inside the VM", "error", err)
				return 1
			}

			sharePWD, err := password.Generate(16, 10, 0, false, false)
			if err != nil {
				slog.Error("Failed to generate ephemeral password for network file share", "error", err)
				return 1
			}

			shareURI := "ftp://linsk:" + sharePWD + "@localhost:" + fmt.Sprint(networkSharePort)

			fmt.Fprintf(os.Stderr, "================\n[Network File Share Config]\nThe network file share was started. Please use the credentials below to connect to the file server.\n\nType: FTP\nServer Address: ftp://localhost:%v\nUsername: linsk\nPassword: %v\n\nShare URI: %v\n================\n", networkSharePort, sharePWD, shareURI)

			err = fm.StartFTP([]byte(sharePWD), networkSharePort+1, ftpPassivePortCount)
			if err != nil {
				slog.Error("Failed to start FTP server", "error", err)
				return 1
			}

			slog.Info("Started the network share successfully", "type", "ftp")

			<-ctx.Done()
			return 0
		}, ports, unrestrictedNetworkingFlag))
	},
}

var luksFlag bool

func init() {
	runCmd.Flags().BoolVarP(&luksFlag, "luks", "l", false, "Use cryptsetup to open a LUKS volume (password will be prompted)")
}

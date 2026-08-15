package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/discovery"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/output"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/sshoptions"
)

func (s *commandState) newAddCommand() *cobra.Command {
	var protocol string
	var host string
	var port uint16
	var user string
	var description string
	var tags []string
	var identityFile string
	var proxyJump string
	var rdpClient string
	var rdpFullscreen bool
	var rdpIgnoreCertificate bool

	command := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a connection to the local catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path, err := s.path()
			if err != nil {
				return err
			}
			sshPath, err := s.sshPath()
			if err != nil {
				return err
			}
			sshCatalog, err := catalog.Build(cmd.Context(), discovery.NewSSHConfigSource(sshPath))
			if err != nil {
				return err
			}
			if _, exists := sshCatalog.Find(strings.TrimSpace(name)); exists {
				return fmt.Errorf("connection %q already exists", name)
			}

			entry := config.Entry{
				Name:        name,
				Protocol:    protocol,
				Host:        host,
				User:        user,
				Description: description,
				Tags:        tags,
			}
			if port != 0 {
				entry.Port = &port
			}

			connection, err := normalizeEntry(entry, path)
			if err != nil {
				return err
			}
			sshFlagsChanged := cmd.Flags().Changed("identity-file") || cmd.Flags().Changed("proxy-jump")
			rdpFlagsChanged := cmd.Flags().Changed("rdp-client") ||
				cmd.Flags().Changed("rdp-fullscreen") ||
				cmd.Flags().Changed("rdp-ignore-certificate")

			switch connection.Endpoint.Protocol {
			case domain.ProtocolSSH:
				if rdpFlagsChanged {
					return fmt.Errorf("RDP options require protocol %q", domain.ProtocolRDP)
				}
				if sshFlagsChanged {
					entry.Options = domain.Options{sshoptions.Namespace: {}}
					if identityFile != "" {
						entry.Options["ssh"]["identity_file"] = identityFile
					}
					if proxyJump != "" {
						entry.Options["ssh"]["proxy_jump"] = proxyJump
					}
				}
			case domain.ProtocolRDP:
				if sshFlagsChanged {
					return fmt.Errorf("SSH options require protocol %q", domain.ProtocolSSH)
				}
				if rdpFlagsChanged {
					if cmd.Flags().Changed("rdp-client") && strings.TrimSpace(rdpClient) == "" {
						return fmt.Errorf("RDP option %q cannot be empty", rdpoptions.Client)
					}
					entry.Options = rdpoptions.Encode(rdpoptions.Options{
						Client:            rdpClient,
						Fullscreen:        rdpFullscreen,
						IgnoreCertificate: rdpIgnoreCertificate,
					})
				}
			default:
				if sshFlagsChanged {
					return fmt.Errorf("SSH options require protocol %q", domain.ProtocolSSH)
				}
				if rdpFlagsChanged {
					return fmt.Errorf("RDP options require protocol %q", domain.ProtocolRDP)
				}
			}

			connection.Options = domain.CloneOptions(entry.Options)
			if err := validateProtocolOptions(connection); err != nil {
				return err
			}
			if err := config.Update(cmd.Context(), path, func(file *config.File) error {
				if connectionNameExists(file.Connections, connection.Name) {
					return fmt.Errorf("connection %q already exists", connection.Name)
				}
				file.Connections = append(file.Connections, config.EntryFromConnection(connection))
				return nil
			}); err != nil {
				return err
			}
			return output.WriteConnection(s.dependencies.Output, connection)
		},
	}
	flags := command.Flags()
	flags.StringVar(&protocol, "protocol", "", "connection protocol (for example ssh, rdp, or vnc)")
	flags.StringVar(&host, "host", "", "connection host")
	flags.Uint16Var(&port, "port", 0, "connection port; defaults to the protocol port")
	flags.StringVar(&user, "user", "", "optional connection user")
	flags.StringVar(&description, "description", "", "optional connection description")
	flags.StringSliceVar(&tags, "tag", nil, "connection tag; may be repeated")
	flags.StringVar(&identityFile, "identity-file", "", "SSH identity file")
	flags.StringVar(&proxyJump, "proxy-jump", "", "SSH proxy jump target")
	flags.StringVar(&rdpClient, "rdp-client", "", "RDP client (currently xfreerdp)")
	flags.BoolVar(&rdpFullscreen, "rdp-fullscreen", false, "start RDP in fullscreen")
	flags.BoolVar(&rdpIgnoreCertificate, "rdp-ignore-certificate", false, "INSECURE: ignore the RDP server certificate")
	return command
}

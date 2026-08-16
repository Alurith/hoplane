package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/output"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

func (s *commandState) newAddCommand() *cobra.Command {
	var protocol string
	var host string
	var port uint16
	var user string
	var description string
	var tags []string
	var rdpClient string
	var rdpDomain string
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
			rdpFlagsChanged := cmd.Flags().Changed("rdp-client") ||
				cmd.Flags().Changed("rdp-domain") ||
				cmd.Flags().Changed("rdp-fullscreen") ||
				cmd.Flags().Changed("rdp-ignore-certificate")

			switch connection.Endpoint.Protocol {
			case domain.ProtocolSSH:
				if rdpFlagsChanged {
					return fmt.Errorf("RDP options require protocol %q", domain.ProtocolRDP)
				}
			case domain.ProtocolRDP:
				if rdpFlagsChanged {
					if cmd.Flags().Changed("rdp-client") && strings.TrimSpace(rdpClient) == "" {
						return fmt.Errorf("RDP option %q cannot be empty", rdpoptions.Client)
					}
					if cmd.Flags().Changed("rdp-domain") && strings.TrimSpace(rdpDomain) == "" {
						return fmt.Errorf("RDP option %q cannot be empty", rdpoptions.Domain)
					}
					entry.Options = rdpoptions.Encode(rdpoptions.Options{
						Client:            rdpClient,
						Domain:            rdpDomain,
						Fullscreen:        rdpFullscreen,
						IgnoreCertificate: rdpIgnoreCertificate,
					})
				}
			default:
				return fmt.Errorf("unsupported protocol %q", connection.Endpoint.Protocol)
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
	flags.StringVar(&protocol, "protocol", "", "connection protocol (ssh or rdp)")
	flags.StringVar(&host, "host", "", "connection host")
	flags.Uint16Var(&port, "port", 0, "connection port; defaults to the protocol port")
	flags.StringVar(&user, "user", "", "optional connection user")
	flags.StringVar(&description, "description", "", "optional connection description")
	flags.StringSliceVar(&tags, "tag", nil, "connection tag; may be repeated")
	flags.StringVar(&rdpClient, "rdp-client", "", "logical RDP client ID (Linux default: xfreerdp3)")
	flags.StringVar(&rdpDomain, "rdp-domain", "", "RDP authentication domain (for example: CONTOSO or the remote computer name)")
	flags.BoolVar(&rdpFullscreen, "rdp-fullscreen", false, "start RDP in fullscreen")
	flags.BoolVar(&rdpIgnoreCertificate, "rdp-ignore-certificate", false, "INSECURE: ignore the RDP server certificate")
	return command
}

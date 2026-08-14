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
			file, err := config.Load(path)
			if err != nil {
				return err
			}
			if connectionNameExists(file.Connections, name) {
				return fmt.Errorf("connection %q already exists", name)
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
			if identityFile != "" || proxyJump != "" {
				entry.Options = domain.Options{"ssh": {}}
				if identityFile != "" {
					entry.Options["ssh"]["identity_file"] = identityFile
				}
				if proxyJump != "" {
					entry.Options["ssh"]["proxy_jump"] = proxyJump
				}
			}
			if port != 0 {
				entry.Port = &port
			}

			connection, err := normalizeEntry(entry, path)
			if err != nil {
				return err
			}
			if len(entry.Options) > 0 && connection.Endpoint.Protocol != domain.ProtocolSSH {
				return fmt.Errorf("SSH options require protocol %q", domain.ProtocolSSH)
			}
			file.Connections = append(file.Connections, config.EntryFromConnection(connection))
			if err := config.Save(path, file); err != nil {
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
	return command
}

//go:build !linux

package connector

func platformRDPClients() (string, map[string]rdpClient) {
	return "", nil
}

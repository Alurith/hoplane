//go:build linux

package connector

func platformRDPClients() (string, map[string]rdpClient) {
	client := xfreerdp3Client{}
	return client.ID(), map[string]rdpClient{
		client.ID(): client,
	}
}

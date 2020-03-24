package serverlist

import (
	"testing"

	"github.com/mochen302/agollo/env"
	"github.com/mochen302/agollo/env/config"
	. "github.com/tevid/gohamcrest"
)

func TestSyncServerIPList(t *testing.T) {
	trySyncServerIPList(t)
}

func trySyncServerIPList(t *testing.T) {
	server := runMockServicesConfigServer()
	defer server.Close()

	newAppConfig := getTestAppConfig()
	newAppConfig.IP = server.URL
	err := SyncServerIPList(newAppConfig)

	Assert(t, err, NilVal())

	servers := env.GetServers()
	serverLen := 0
	servers.Range(func(k, v interface{}) bool {
		serverLen++
		return true
	})

	Assert(t, 10, Equal(serverLen))

}

func getTestAppConfig() *config.AppConfig {
	jsonStr := `{
    "appId": "test",
    "cluster": "dev",
    "namespaceName": "application",
    "ip": "localhost:8888",
    "releaseKey": "1"
	}`
	c, _ := env.Unmarshal([]byte(jsonStr))

	return c.(*config.AppConfig)
}

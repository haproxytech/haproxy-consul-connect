package state

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/haproxytech/haproxy-consul-connect/haproxy/haproxy_cmd"
	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/stretchr/testify/require"
)

func testCfg(t *testing.T, cfgDir string, state State) {
	sd := lib.NewShutdown()
	defer sd.Shutdown("test end")
	go func() {
		<-sd.Stop
		os.RemoveAll(cfgDir)
	}()

	haSock := cfgDir + "/hasock"

	cfg := `
	global
		master-worker
		stats socket ` + haSock + ` mode 600 level admin expose-fd listeners
		stats timeout 2m
		tune.ssl.default-dh-param 1024
		nbproc 1
		nbthread 1

	userlist controller
		user usr insecure-password pass
	`

	haCfgPath := cfgDir + "/cfg"

	err := ioutil.WriteFile(haCfgPath, []byte(cfg), 0644)
	require.NoError(t, err)

	err = ioutil.WriteFile(cfgDir+"/cert", []byte(`
-----BEGIN CERTIFICATE-----
MIIChzCCAi2gAwIBAgIBCDAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg
Q0EgNzAeFw0xOTEwMDIxMzU4MjhaFw0xOTEwMDUxMzU4MjhaMA4xDDAKBgNVBAMT
A3dlYjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABNZnzOGBxy8SzZVa/ClMXRQy
/53U9xaTuEU3o24oC+a4yyZnj4Rn9HL5UsP2HxfD8Dj0x6wOqaadkTgaDM2yPkej
ggFyMIIBbjAOBgNVHQ8BAf8EBAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG
AQUFBwMBMAwGA1UdEwEB/wQCMAAwaAYDVR0OBGEEXzExOjNiOjdhOjg5OjI5OjU1
OmM5OjExOjI3OmNhOmMzOjFiOmU5OjkzOmE3OjYxOjEzOjcwOjhmOmYzOjE2OmEy
OjIyOjcyOjkxOjZhOjdiOjg1OmNmOjAyOjhhOjQyMGoGA1UdIwRjMGGAXzExOjNi
OjdhOjg5OjI5OjU1OmM5OjExOjI3OmNhOmMzOjFiOmU5OjkzOmE3OjYxOjEzOjcw
OjhmOmYzOjE2OmEyOjIyOjcyOjkxOjZhOjdiOjg1OmNmOjAyOjhhOjQyMFkGA1Ud
EQRSMFCGTnNwaWZmZTovL2E5OWY4ZTUwLWU5YzAtMzRhMS1hM2RkLTQwYWFmODU2
OTJmYS5jb25zdWwvbnMvZGVmYXVsdC9kYy9kYzEvc3ZjL3dlYjAKBggqhkjOPQQD
AgNIADBFAiEAz5P62jXfeRbdg1JG7now5tQ2p02np/JjRydMd1+ky1MCIE95qdtS
HtO1OOIZ3tn0iJvnlebi11leakhKR9pL460P
-----END CERTIFICATE-----
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIJid2pKnv2d/BJxoU5iE1ApyUWQPQlVb9J4HqUj6MPy6oAoGCCqGSM49
AwEHoUQDQgAE1mfM4YHHLxLNlVr8KUxdFDL/ndT3FpO4RTejbigL5rjLJmePhGf0
cvlSw/YfF8PwOPTHrA6ppp2ROBoMzbI+Rw==
-----END EC PRIVATE KEY-----
	`), 0644)
	require.NoError(t, err)

	err = ioutil.WriteFile(cfgDir+"/ca", []byte(`
-----BEGIN CERTIFICATE-----
MIICWTCCAf+gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg
Q0EgNzAeFw0xOTEwMDIxMzU5MjhaFw0yOTEwMDIxMzU5MjhaMBYxFDASBgNVBAMT
C0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEnsjeSq6LRi9J
+t4PXDVtkCgX1rN8rXQazXxsFMowOWkS7122t9FGUD4XR6gNJCwx+VE9+8XqUzaN
5+C3mq90WKOCATwwggE4MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/
MGgGA1UdDgRhBF8xMTozYjo3YTo4OToyOTo1NTpjOToxMToyNzpjYTpjMzoxYjpl
OTo5MzphNzo2MToxMzo3MDo4ZjpmMzoxNjphMjoyMjo3Mjo5MTo2YTo3Yjo4NTpj
ZjowMjo4YTo0MjBqBgNVHSMEYzBhgF8xMTozYjo3YTo4OToyOTo1NTpjOToxMToy
NzpjYTpjMzoxYjplOTo5MzphNzo2MToxMzo3MDo4ZjpmMzoxNjphMjoyMjo3Mjo5
MTo2YTo3Yjo4NTpjZjowMjo4YTo0MjA/BgNVHREEODA2hjRzcGlmZmU6Ly9hOTlm
OGU1MC1lOWMwLTM0YTEtYTNkZC00MGFhZjg1NjkyZmEuY29uc3VsMAoGCCqGSM49
BAMCA0gAMEUCIQDW7TYjr/zByA6tsFn3ETLI8B2tAtxgskJL1MKwAtPtFwIgdDdB
RHmDi0qnL6qrKfjTOnfHgQPCgxAy9knMIiDzBRg=
-----END CERTIFICATE-----

	`), 0644)
	require.NoError(t, err)

	err = ioutil.WriteFile(cfgDir+"/spoe", []byte(`
	[intentions]

	spoe-agent intentions-agent
		messages check-intentions

		option var-prefix connect

		timeout hello      3000ms
		timeout idle       3000s
		timeout processing 3000ms

		use-backend spoe_back

	spoe-message check-intentions
		args ip=src cert=ssl_c_der
		event on-frontend-tcp-request
	`), 0644)
	require.NoError(t, err)

	dp, err := haproxy_cmd.Start(sd, haproxy_cmd.Config{
		HAProxyPath:             "haproxy",
		HAProxyConfigPath:       haCfgPath,
		DataplanePath:           "dataplaneapi",
		DataplaneTransactionDir: cfgDir,
		DataplaneSock:           cfgDir + "/dpsock",
		DataplaneUser:           "usr",
		DataplanePass:           "pass",
	})
	require.NoError(t, err)

	tx := dp.Tnx()

	err = Apply(tx, State{}, state)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	current, err := FromHAProxy(dp)
	require.NoError(t, err)
	require.Equal(t, len(state.Backends), len(current.Backends))
	require.Equal(t, len(state.Frontends), len(current.Frontends))

	// Sort to be sure order is predictible
	sort.Sort(Frontends(state.Frontends))
	sort.Sort(Frontends(current.Frontends))
	// Sort to be sure order is predictible
	sort.Sort(Backends(state.Backends))
	sort.Sort(Backends(current.Backends))

	require.Equal(t, state.Backends, current.Backends)
	require.Equal(t, state.Frontends, current.Frontends)
	require.Equal(t, state, current)
}

func TestFromHA(t *testing.T) {
	cfgDir, err := ioutil.TempDir("", fmt.Sprintf("%s_*", t.Name()))
	require.NoError(t, err)

	state := GetTestHAConfig(cfgDir)
	testCfg(t, cfgDir, state)
}

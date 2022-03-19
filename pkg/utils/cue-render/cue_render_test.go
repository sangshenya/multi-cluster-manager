package cur_render

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/coredns/caddy"
	"github.com/coredns/caddy/caddyfile"

	v1 "k8s.io/api/core/v1"
)

const endpointCue = `
	output: endpoints: [
		for _, v in {
			if context.subsets != _|_ && len(context.subsets) > 0 {
				for subset in context.subsets {
					if subset.addresses != _|_ && len(subset.addresses) > 0 && subset.ports != _|_ && len(subset.ports) > 0 {
						for i, addr in subset.addresses {
							for j, po in subset.ports {
								{
									"\(i)-\(j)": addr.ip + ":\(po.port)"
								},
							}
						}
					}
				}
			}
		} {v}
	]
`

func TestCueRender(t *testing.T) {
	ep := &v1.Endpoints{}
	ep.APIVersion = "v1"
	ep.Name = "test"
	ep.Namespace = "chenkun"
	ep.Kind = "Endpoints"
	ep.Subsets = []v1.EndpointSubset{
		v1.EndpointSubset{
			Addresses: []v1.EndpointAddress{
				v1.EndpointAddress{
					IP: "10.244.0.100",
				},
				v1.EndpointAddress{
					IP: "10.244.154.81",
				},
			},
			Ports: []v1.EndpointPort{
				v1.EndpointPort{
					Port: 9200,
				},
				v1.EndpointPort{
					Port: 9300,
				},
			},
		},
	}

	data, err := RenderCue(ep, endpointCue, "output")
	if err != nil {
		fmt.Println("apply cue error:", err.Error())
		return
	}
	fmt.Println(string(data))
}

type corednsPluginConfigModel struct {
	EnableErrorLogging bool       `json:"enableErrorLogging"`
	CacheTime          int        `json:"cacheTime"`
	Hosts              []DnsModel `json:"hosts,omitempty"`
	Forward            []DnsModel `json:"forward,omitempty"`
}

type DnsModel struct {
	Domain     string   `json:"domain"`
	Resolution []string `json:"resolution"`
}

func TestCoreDnsConfig(t *testing.T) {
	caddy.DefaultConfigFile = "/Users/chenkun/Downloads/coredns-1.8.0/corefile"
	caddy.SetDefaultCaddyfileLoader("default", caddy.LoaderFunc(defaultLoader))
	// Get Corefile input
	corefile, err := caddy.LoadCaddyfile("dns")
	if err != nil {
		fmt.Println(err)
		return
	}
	validDirectives := caddy.ValidDirectives("dns")
	serverBlocks, err := caddyfile.Parse(corefile.Path(), bytes.NewReader(corefile.Body()), validDirectives)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(serverBlocks) == 0 {
		fmt.Println("serverBlocks is empty")
		return
	}
	pluginConfigModel := &corednsPluginConfigModel{}
	for _, serverBlock := range serverBlocks {
		for key, value := range serverBlock.Tokens {
			switch key {
			case "errors":
				pluginConfigModel.EnableErrorLogging = true
			case "cache":
				if len(value) >= 2 && value[0].Text == "cache" {
					cacheTime, err := strconv.Atoi(value[1].Text)
					if err == nil {
						pluginConfigModel.CacheTime = cacheTime
					}
				}
			case "forward":
				dnsModel := DnsModel{}
				if len(value) < 3 {
					return
				}
				dnsModel.Domain = value[1].Text
				for i := 2; i < len(value); i++ {
					dnsModel.Resolution = append(dnsModel.Resolution, value[i].Text)
				}
				pluginConfigModel.Forward = append(pluginConfigModel.Forward, dnsModel)
			case "hosts":
				dnsModel := DnsModel{}
				if len(value) < 3 {
					return
				}
				dnsModel.Domain = value[1].Text
				for i := 2; i < len(value); i++ {
					dnsModel.Resolution = append(dnsModel.Resolution, value[i].Text)
				}
				pluginConfigModel.Hosts = append(pluginConfigModel.Hosts, dnsModel)
			}
		}
	}

	fmt.Println(pluginConfigModel)
}

// defaultLoader loads the Corefile from the current working directory.
func defaultLoader(serverType string) (caddy.Input, error) {
	contents, err := ioutil.ReadFile(caddy.DefaultConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return caddy.CaddyfileInput{
		Contents:       contents,
		Filepath:       caddy.DefaultConfigFile,
		ServerTypeName: serverType,
	}, nil
}

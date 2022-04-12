package cur_render

import (
	"fmt"
	"testing"

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

package addons_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"harmonycloud.cn/stellaris/pkg/agent/addons"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/agent"
)

var _ = Describe("Addons", func() {
	var (
		inTree      []model.In
		outTree     []model.Out
		plugins     model.Plugins
		addonConfig model.PluginsConfig

		addonsInfoExcept []model.Addon
		requestExcept    *model.RegisterRequest
	)

	Describe("Addons starting", func() {
		Context("Load", func() {
			BeforeEach(func() {

			})
			JustBeforeEach(func() {

			})
			It("Get addons config", func() {
				in := model.In{Name: "addon1"}
				out := model.Out{Name: "addon2", Url: "www.123.com"}
				inTree = append(inTree, in)
				outTree = append(outTree, out)
				plugins = model.Plugins{InTree: inTree, OutTree: outTree}
				addonConfig = model.PluginsConfig{Plugins: plugins}

				res, _ := agent.GetAddonConfig("path/to/test.yaml")
				Expect(res).To(Equal(&addonConfig))
			})
			It("Load addons,will run thread for each plugin,and write data into a channel", func() {
				inTreeProperties := make(map[string]string)
				inTreeProperties["inTree"] = "addon1"
				inTreeRes := model.Addon{Name: "addon1", Properties: inTreeProperties}
				outTreeProperties := make(map[string]string)
				outTreeProperties["outTree"] = "www.123.com"
				outTreeRes := model.Addon{Name: "addon2", Properties: outTreeProperties}
				addonsInfoExcept = append(addonsInfoExcept, inTreeRes)
				addonsInfoExcept = append(addonsInfoExcept, outTreeRes)
				requestExcept = &model.RegisterRequest{Addons: addonsInfoExcept}

				registerRequest, _, _ := addons.Load(&addonConfig)
				Expect(registerRequest).To(Equal(requestExcept))

			})
		})
	})
})

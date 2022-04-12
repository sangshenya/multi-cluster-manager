package outTree

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/utils/httprequest"
)

type OutTreeResponseModel struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    *OutTreeData `json:"data,omitempty"`
}

type OutTreeData struct {
	Data []byte `json:"data"`
}

// LoadOutTreeData load outTree plugins data
func LoadOutTreeData(ctx context.Context, out *model.Out) (*model.AddonsData, error) {

	var infos model.CommonAddonInfo
	for _, url := range out.Configurations.HTTP.URL {
		info := requestDataWithURL(url)
		infos.Info = append(infos.Info, info)
	}

	return &model.AddonsData{
		Name: out.Name,
		Info: infos,
	}, nil
}

func requestDataWithURL(url string) model.AddonsInfo {
	info := model.AddonsInfo{}
	info.Type = model.AddonInfoSourceStatic
	info.Address = url
	info.Status = model.AddonStatusTypeNotReady

	response, err := httprequest.HttpGetWithEmptyHeader(url)
	if err != nil {
		info.Data = err.Error()
		return info
	}
	if response.StatusCode != 200 {
		info.Data = "status code is not 200"
		return info
	}
	data, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		info.Data = err.Error()
		return info
	}

	outTreeData := &OutTreeResponseModel{}
	err = json.Unmarshal(data, outTreeData)
	if err != nil {
		info.Data = err.Error()
		return info
	}

	if outTreeData.Code == 0 {
		info.Status = model.AddonStatusTypeReady
	}
	if outTreeData.Data != nil {
		info.Data = outTreeData.Data
	}
	return info
}

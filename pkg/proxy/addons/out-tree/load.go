package outTree

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"

	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/utils/httprequest"
)

type OutTreeResponseModel struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []byte `json:"data,omitempty"`
}

// load outTree plugins data
func LoadOutTreeData(ctx context.Context, out *model.Out) (*model.AddonsData, error) {
	response, err := httprequest.HttpGetWithEmptyHeader(out.Http.Url)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, errors.New("status code is not 200")
	}
	data, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, err
	}

	outTreeData := &OutTreeResponseModel{}
	err = json.Unmarshal(data, outTreeData)
	if err != nil {
		return nil, err
	}

	addonsInfo := model.AddonsInfo{
		Type:    model.AddonInfoSourceStatic,
		Address: out.Http.Url,
		Status:  model.AddonStatusTypeNotReady,
	}

	if outTreeData.Code == 0 {
		addonsInfo.Status = model.AddonStatusTypeReady
	}

	return &model.AddonsData{
		Name: out.Name,
		Info: []model.AddonsInfo{
			addonsInfo,
		},
	}, nil
}

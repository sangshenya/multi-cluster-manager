package outTree

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/google/uuid"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/httprequest"
)

// load outTree plugins data
func LoadOutTreeData(url string) (*model.PluginsData, error) {
	response, err := httprequest.HttpGetWithEmptyHeader(url)
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

	outTreeData := &model.PluginsData{}
	err = json.Unmarshal(data, outTreeData)
	if err != nil {
		return nil, err
	}
	if len(outTreeData.Uid) <= 0 {
		outTreeData.Uid = uuid.NewString()
	}
	return outTreeData, nil
}

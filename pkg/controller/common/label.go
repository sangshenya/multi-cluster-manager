package common

import (
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"harmonycloud.cn/stellaris/pkg/util/common"
)

func GenerateLabelKey(k string, v string) (string, error) {
	mappingK, err := common.GenerateNameByOption(k, v, "-")
	if err != nil {
		return "", err
	}
	labelK, err := common.GenerateName(managerCommon.NamespaceMappingLabel, mappingK)
	if err != nil {
		return "", err
	}
	return labelK, nil
}

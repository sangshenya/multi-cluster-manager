package common

import (
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
)

func GenerateLabelKey(k string, v string) (string, error) {
	return managerCommon.NamespaceMappingLabel + k + "_" + v, nil
}

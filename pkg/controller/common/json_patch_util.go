package common

import (
	"encoding/json"
	"errors"

	jsonpatch "github.com/evanphx/json-patch"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	"k8s.io/apimachinery/pkg/runtime"
)

func ApplyJsonPatch(resource *runtime.RawExtension,
	override []common.JSONPatch) (*runtime.RawExtension, error) {
	if resource == nil {
		return nil, errors.New("resource is empty")
	}
	if override == nil {
		return resource, nil
	}
	jsonPatchBytes, err := json.Marshal(override)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return nil, err
	}
	patchedObjectJsonBytes, err := patch.Apply(resource.Raw)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: patchedObjectJsonBytes}, nil
}

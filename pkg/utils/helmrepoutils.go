package utils

import (
	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/repo"
)

//LoadIndex loads data into a repo.IndexFile
func LoadIndex(data []byte) (*repo.IndexFile, error) {
	i := &repo.IndexFile{}
	if err := yaml.Unmarshal(data, i); err != nil {
		return i, err
	}
	i.SortEntries()
	if i.APIVersion == "" {
		return i, repo.ErrNoAPIVersion
	}
	return i, nil
}

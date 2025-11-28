package es

type IndexInfo struct {
	Name         string `json:"index"`
	Health       string `json:"health"`
	Status       string `json:"status"`
	UUID         string `json:"uuid"`
	Pri          string `json:"pri"`
	Rep          string `json:"rep"`
	Count        string `json:"docs.count"`
	Deleted      string `json:"docs.deleted"`
	StoreSize    string `json:"store.size"`
	PriStoreSize string `json:"pri.store.size"`
}

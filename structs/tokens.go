package structs

type HFToken struct {
	Name  string `json:"name"`
	Value string `json:"value"` // always encrypted in DB
}

type HFTokenRequest struct {
	Name    string `json:"name"`
	HFToken string `json:"hf_token"`
}

type RemoveHFTokenRequest struct {
	Name string `json:"name"`
}

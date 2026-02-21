package structs

type UserCreate struct {
	Username string   `json:"username" binding:"required"`
	Password string   `json:"password" binding:"required"`
	HFToken  []string `json:"hf_token"`
}

type UserLogin struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

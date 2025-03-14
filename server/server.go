package server

type Server struct {
	Id string `json:"id"`
	Uuid string `json:"uuid"`
	Name string `json:"name"`
	Description string `json:"description"`

	Template int `json:"template"`

	Network Network `json:"network"`
}

type Network struct {
	Allocations []Allocation `json:"allocations"`

	Memory int `json:"memory"`
	Cpu    int `json:"cpu"`
}

type Allocation struct {
	Ip   string `json:"ip"`
	Port int    `json:"port"`
}

package internal

type Secret struct {
	ApiKey, ZoneId string
}

type RecordResponse struct {
	Records [] Record 	`json:"records"`
	Meta Meta 			`json:"meta"`
}

type Meta struct {
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page int 			`json:"page"`
	PerPage int 		`json:"per_page"`
	LastPage int		`json:"last_page"`
	TotalEntries int	`json:"total_entries"`
}

type Record struct {
	Type string 		`json:"type"`
	Id string			`json:"id"`
	Created string 		`json:"created"`
	Modified string		`json:"modified"`
	ZoneId string		`json:"zone_id"`
	Name string			`json:"name"`
	Value string		`json:"value"`
	Ttl int				`json:"ttl"`
}
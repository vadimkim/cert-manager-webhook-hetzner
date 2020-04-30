package internal

type Config struct {
	ApiKey, ZoneName, ApiUrl string
}

type RecordResponse struct {
	Records []Record `json:"records"`
	Meta    Meta     `json:"meta"`
}

type ZoneResponse struct {
	Zones []Zone `json:"zones"`
	Meta  Meta   `json:"meta"`
}

type Meta struct {
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page         int `json:"page"`
	PerPage      int `json:"per_page"`
	LastPage     int `json:"last_page"`
	TotalEntries int `json:"total_entries"`
}

type Record struct {
	Type     string `json:"type"`
	Id       string `json:"id"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
	ZoneId   string `json:"zone_id"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Ttl      int    `json:"ttl"`
}

type Zone struct {
	Id              string       `json:"id"`
	Created         string       `json:"created"`
	Modified        string       `json:"modified"`
	LegacyDnsHost   string       `json:"legacy_dns_host"`
	LegacyNs        []string     `json:"legacy_ns"`
	Name            string       `json:"name"`
	Ns              []string     `json:"ns"`
	Owner           string       `json:"owner"`
	Paused          bool         `json:"paused"`
	Permission      string       `json:"permission"`
	Project         string       `json:"project"`
	Registrar       string       `json:"registrar"`
	Status          string       `json:"status"`
	Ttl             int          `json:"ttl"`
	Verified        string       `json:"verified"`
	RecordsCount    int          `json:"records_count"`
	IsSecondaryDns  bool         `json:"is_secondary_dns"`
	TxtVerification Verification `json:"txt_verification"`
}

type Verification struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

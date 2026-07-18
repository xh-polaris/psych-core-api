package conf

type COS struct {
	BucketURL string
	CDN       string `json:",optional"`
	SecretID  string
	SecretKey string
}

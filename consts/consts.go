package consts

const (
	TimeFormatLayout string = "2006-01-02 15:04:05.000"
	LoggerYamlPath string = "./config/logger.yaml"
	TwitterYamlPath string = "./config/twitter.yaml"
	MgoYamlPath string = "./config/mgo.yaml"
	RequestTokenUrl string = "http://api.twitter.com/oauth/request_token"
	AuthorizeTokenUrl string = "https://api.twitter.com/oauth/authorize"
	AccessTokenUrl string = "https://api.twitter.com/oauth/access_token"
	SearchUrl string = "https://api.twitter.com/1.1/search/tweets.json"
)

var SearchParams = []map[string]string{
	map[string]string{"q": "\"go言語\"+AND+\"http://\"", "result_type": "recent", "count": "100"},
	map[string]string{"q": "\"go言語\"+AND+\"https://\"", "result_type": "recent", "count": "100"},
	map[string]string{"q": "\"golang\"+AND+\"http://\"", "result_type": "recent", "count": "100"},
	map[string]string{"q": "\"golang\"+AND+\"https://\"", "result_type": "recent", "count": "100"},
}

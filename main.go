package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/mrjones/oauth"
	"github.com/yosssi/goes/consts"
	"github.com/yosssi/gologger"
	"github.com/yosssi/goutils"
	"io/ioutil"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"launchpad.net/goyaml"
	"strings"
	"time"
)

type Twitter struct {
	Consumer    *oauth.Consumer
	AccessToken *oauth.AccessToken
}

type Url struct {
	Id          bson.ObjectId `bson:"_id"`
	RawUrl      string        `bson:"RawUrl"`
	ExpandedUrl string        `bson:"ExpandedUrl"`
	CreatedAt   time.Time     `bson:"CreatedAt"`
	UpdatedAt   time.Time     `bson:"UpdatedAt"`
}

type Link struct {
	Id        bson.ObjectId `bson:"_id"`
	Url       string        `bson:"Url"`
	Title     string        `bson:"Title"`
	CreatedAt time.Time     `bson:"CreatedAt"`
	UpdatedAt time.Time     `bson:"UpdatedAt"`
}

var (
	loggerYaml  map[string]string = make(map[string]string)
	twitterYaml map[string]string = make(map[string]string)
	mgoYaml     map[string]string = make(map[string]string)
	logger      gologger.Logger
	twitter     Twitter
	urls        map[string]string
)

func (t *Twitter) Get(url string, params map[string]string) (interface{}, error) {
	response, err := t.Consumer.Get(url, params, t.AccessToken)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	defer response.Body.Close()
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(b, &result)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	return result, err
}

func main() {
	initialize()
	for {
		search()
		insertUrls()
		setExpandedUrls()
		setTitles()
		sleep()
	}
}

func initialize() {
	print("Start initailize.")
	setYaml("loggerYaml", consts.LoggerYamlPath, loggerYaml)
	setLogger()
	setYaml("twitterYaml", consts.TwitterYamlPath, twitterYaml)
	setTwitter()
	setYaml("mgoYaml", consts.MgoYamlPath, mgoYaml)
	logger.Info("End Initialize.")
}

func now() string {
	return time.Now().Format(consts.TimeFormatLayout)
}

func print(s ...interface{}) {
	fmt.Print(now(), " - ")
	fmt.Println(s...)
}

func setYaml(name string, filePath string, yaml map[string]string) {
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	err = goyaml.Unmarshal(bytes, &yaml)
	if err != nil {
		panic(err)
	}
	if logger.Name != "" {
		logger.Info(name, "was set.", "yaml:", yaml)
	} else {
		print(name, "was set.", "yaml:", yaml)
	}
}

func setLogger() {
	logger = gologger.GetLogger(loggerYaml)
	logger.Info("logger was set.", "logger:", goutils.StructToMap(&logger))
}

func setTwitter() {
	twitter = Twitter{}
	twitter.Consumer = oauth.NewConsumer(
		twitterYaml["ConsumerKey"],
		twitterYaml["ConsumerSecret"],
		oauth.ServiceProvider{
			RequestTokenUrl:   consts.RequestTokenUrl,
			AuthorizeTokenUrl: consts.AuthorizeTokenUrl,
			AccessTokenUrl:    consts.AccessTokenUrl,
		},
	)
	twitter.AccessToken = &oauth.AccessToken{twitterYaml["AccessToken"], twitterYaml["AccessTokenSecret"], nil}
	logger.Info("twitter was set.", "twitter:", goutils.StructToMap(&twitter))
}

func search() {
	logger.Info("Start search")
	urls = make(map[string]string)
	for _, params := range consts.SearchParams {
		execSearch(params)
	}
	logger.Info("End search", urls)
}

func execSearch(params map[string]string) {
	logger.Info("Start execSearch", params)
	res, err := twitter.Get(
		consts.SearchUrl,
		params,
	)
	if err != nil {
		logger.Error(err)
	}
	for key, val := range res.(map[string]interface{}) {
		if key == "statuses" {
			data, _ := val.([]interface{})
			for _, tweet := range data {
				tweetUrls := tweet.(map[string]interface{})["entities"].(map[string]interface{})["urls"].([]interface{})
				if len(tweetUrls) > 0 {
					for _, url := range tweetUrls {
						setUrls(url.(map[string]interface{})["expanded_url"].(string))
					}
				}
				tweetMedias := tweet.(map[string]interface{})["entities"].(map[string]interface{})["media"]
				if tweetMedias != nil {
					for _, tweetMedia := range tweetMedias.([]interface{}) {
						tweetMediaUrl := tweetMedia.(map[string]interface{})["url"]
						setUrls(tweetMediaUrl.(string))
					}
				}
			}
			break
		}
	}
	logger.Info("End execSearch")
}

func setUrls(s string) {
	url := goutils.RemoveUtmParams(goutils.RemoveTwitterUrlHash(s))
	_, prs := urls[url]
	if !prs {
		urls[url] = ""
	}
}

func insertUrls() {
	logger.Info("insertUrls starts.")
	session, err := mgo.Dial(mgoYaml["Host"])
	if err != nil {
		panic(err)
	}
	defer session.Close()
	session.SetMode(mgo.Monotonic, true)
	c := session.DB(mgoYaml["Db"]).C("urls")
	for rawUrl, _ := range urls {
		url := Url{}
		err = c.Find(bson.M{"RawUrl": rawUrl}).One(&url)
		if err != nil {
			if err.Error() == mgo.ErrNotFound.Error() {
				now := time.Now()
				url = Url{Id: bson.NewObjectId(), RawUrl: rawUrl, ExpandedUrl: "", CreatedAt: now, UpdatedAt: now}
				err := c.Insert(&url)
				if err != nil {
					panic(err)
				}
				logger.Info("url was inserted.", url)
			} else {
				panic(err)
			}
		} else {
			logger.Info("url existed and was not inserted.", url)
		}
	}
	logger.Info("insertUrls end.")
}

func setExpandedUrls() {
	logger.Info("setExpandedUrls start.")
	session, err := mgo.Dial(mgoYaml["Host"])
	if err != nil {
		panic(err)
	}
	defer session.Close()
	session.SetMode(mgo.Monotonic, true)
	c := session.DB(mgoYaml["Db"]).C("urls")
	urls := make([]Url, 0)
	err = c.Find(bson.M{"ExpandedUrl": ""}).All(&urls)
	if err != nil {
		panic(err)
	}
	for _, url := range urls {
		id := url.Id
		rawUrl := url.RawUrl
		expandedUrl := goutils.RemoveUtmParams(goutils.RemoveTwitterUrlHash(goutils.NormalUrl(rawUrl)))
		if expandedUrl != "" {
			err := c.UpdateId(id, bson.M{"$set": bson.M{"ExpandedUrl": expandedUrl, "UpdatedAt": time.Now()}})
			if err != nil {
				panic(err)
			}
			logger.Info("url was updated.", id, rawUrl, expandedUrl)
			insertLink(session, expandedUrl)
		} else {
			logger.Info("expandedUrl did not exist and url was not updated.", rawUrl)
		}
	}
	logger.Info("setExpandedUrls ends.")
}

func insertLink(session *mgo.Session, url string) {
	link := Link{}
	c := session.DB(mgoYaml["Db"]).C("links")
	err := c.Find(bson.M{"Url": url}).One(&link)
	if err != nil {
		if err.Error() == mgo.ErrNotFound.Error() {
			now := time.Now()
			link = Link{Id: bson.NewObjectId(), Url: url, Title: "", CreatedAt: now, UpdatedAt: now}
			err = c.Insert(&link)
			if err != nil {
				panic(err)
			}
			logger.Info("link was inserted.", url)
		} else {
			panic(err)
		}
	} else {
		logger.Info("link existed and was not inserted.", link)
	}
}

func setTitles() {
	logger.Info("setTitles starts.")
	session, err := mgo.Dial(mgoYaml["Host"])
	if err != nil {
		panic(err)
	}
	defer session.Close()
	session.SetMode(mgo.Monotonic, true)
	c := session.DB(mgoYaml["Db"]).C("links")
	links := make([]Link, 0)
	err = c.Find(bson.M{"Title": ""}).All(&links)
	if err != nil {
		panic(err)
	}
	for _, link := range links {
		id := link.Id
		url := link.Url
		title := getTitle(url)
		err := c.UpdateId(id, bson.M{"$set": bson.M{"Title": title, "UpdatedAt": time.Now()}})
		if err != nil {
			panic(err)
		}
		logger.Info("title was updated.", id, url, title)
	}
	logger.Info("setTitles ends.")
}

func getTitle(url string) string {
	logger.Info("Get title.", url)
	title := ""
	doc, err := goquery.NewDocument(url)
	if err != nil {
		logger.Error(err.Error())
		return ""
	}
	title = strings.Replace(strings.Replace(strings.Replace(strings.TrimSpace(doc.Find("title").Text()), "\r\n", " ", -1), "\r", " ", -1), "\n", " ", -1)
	if title == "" {
		title = url
	}
	return title
}

func sleep() {
	logger.Info("sleep starts.")
	time.Sleep(time.Second * 10)
	logger.Info("sleep ends.")
}

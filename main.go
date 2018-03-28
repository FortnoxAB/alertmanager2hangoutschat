package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"text/template"

	"github.com/fortnoxab/ginprometheus"
	"github.com/gin-gonic/gin"
	"github.com/jonaz/ginlogrus"
	"github.com/jonaz/gograce"
	alerttemplate "github.com/prometheus/alertmanager/template"
	"github.com/sirupsen/logrus"
)

var port = flag.String("port", "8080", "Port to listen to")
var postPath = flag.String("path", "/alertmanager", "What path to listen to for POST requests")
var logFormat = flag.String("log-format", "json", "can be empty string or json")
var logLevel = flag.String("log-level", "info", "Can be one of:"+strings.Join(validLogLevels(), ","))
var templateString = flag.String("template-string", messageTemplate, "template for the messages sent to hangouts chat")

var messageTemplate = `<users/all>
*{{.QueryParams.Get "env" | toUpper }}: {{ .Labels.alertname }} - {{.Status | toUpper}}*
{{ range .Annotations.SortedPairs -}}
{{ .Name }}: {{ .Value}}
{{ end -}}
Source: <{{ .GeneratorURL }}|Show in prometheus>
`

func main() {
	flag.Parse()

	lvl, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Setting logrus logging level to %s\n", lvl)
	logrus.SetLevel(lvl)
	if *logFormat == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	srv, shutdown := gograce.NewServerWithTimeout(5 * time.Second)
	srv.Handler = getWebRouter()
	srv.Addr = ":" + *port

	logrus.Error(srv.ListenAndServe())
	<-shutdown
}

func validLogLevels() []string {
	lvls := make([]string, len(logrus.AllLevels))
	for k, v := range logrus.AllLevels {
		lvls[k] = v.String()
	}
	return lvls
}

func getWebRouter() http.Handler {
	router := gin.New()
	router.Use(ginlogrus.New(logrus.StandardLogger(), "/health", "/metrics"), gin.Recovery())
	m := ginprometheus.New("http")
	m.Use(router)

	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST(*postPath, handleAlert)

	return router
}

// defaultFuncs is copied from alertmanager templates
var defaultFuncs = template.FuncMap{
	"toUpper": strings.ToUpper,
	"toLower": strings.ToLower,
	"title":   strings.Title,
	// join is equal to strings.Join but inverts the argument order
	// for easier pipelining in templates.
	"join": func(sep string, s []string) string {
		return strings.Join(s, sep)
	},
	"reReplaceAll": func(pattern, repl, text string) string {
		re := regexp.MustCompile(pattern)
		return re.ReplaceAllString(text, repl)
	},
}

type alertData struct {
	alerttemplate.Alert
	QueryParams url.Values
}

type textRequest struct {
	Text string `json:"text"`
}

func handleAlert(c *gin.Context) {

	defer c.Request.Body.Close()
	dec := json.NewDecoder(c.Request.Body)

	data := alerttemplate.Data{}
	err := dec.Decode(&data)
	if err != nil {
		logrus.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	for _, alert := range data.Alerts {
		err := sendAlert(alert, c.Request.URL.Query())
		if err != nil {
			c.Error(err)
			continue
		}

	}
	if len(c.Errors.Errors()) > 0 {
		for _, v := range c.Errors {
			logrus.Error(v)
		}
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func sendAlert(data alerttemplate.Alert, getParams url.Values) error {
	hangoutsURL, err := url.Parse(getParams.Get("url"))
	if err != nil {
		return err
	}

	alert := &alertData{
		Alert:       data,
		QueryParams: getParams,
	}
	tmpl, err := generateTemplate(messageTemplate, alert)
	if err != nil {
		return err
	}

	textReq := &textRequest{
		Text: tmpl,
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	err = enc.Encode(&textReq)
	if err != nil {
		return err
	}
	return sendChatMessage(hangoutsURL, buf)
}

func sendChatMessage(u *url.URL, data io.Reader) error {
	req, err := http.NewRequest("POST", u.String(), data)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error from google: %s", string(body))
	}

	return nil
}

func generateTemplate(s string, data interface{}) (string, error) {
	tmpl, err := template.New("").Funcs(defaultFuncs).Parse(messageTemplate)
	if err != nil {
		return "", err
	}
	var to bytes.Buffer
	err = tmpl.Execute(&to, data)
	return to.String(), err
}

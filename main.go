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
var postPath = flag.String("path", "/api/alertmanager2hangoutschat/alertmanager", "What path to listen to for POST requests")
var logFormat = flag.String("log-format", "json", "can be empty string or json")
var logLevel = flag.String("log-level", "info", "Can be one of:"+strings.Join(validLogLevels(), ","))
var templateString = flag.String("template-string", messageTemplate, "template for the messages sent to hangouts chat")

var messageTemplate = `{{ define "print_annotations" }}{{ range . }}
*{{ .Labels.alertname }}*
{{ range .Annotations.SortedPairs -}}
{{ .Name }}: {{ .Value}}
{{ end -}}
Source: <{{ .GeneratorURL }}|Show in prometheus>
{{ end -}}{{ end -}}
<users/all>
*{{.QueryParams.Get "env" | toUpper }} - {{ .Status | toUpper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}*
{{ if gt (len .Alerts.Firing) 0 -}}
{{ template "print_annotations" .Alerts.Firing -}}
{{ end -}}
{{ if gt (len .Alerts.Resolved) 0 -}}
{{ template "print_annotations" .Alerts.Resolved -}}
{{ end -}}
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
	alerttemplate.Data
	QueryParams url.Values
}

type textRequest struct {
	Text string `json:"text"`
}

func handleAlert(c *gin.Context) {

	defer c.Request.Body.Close()
	dec := json.NewDecoder(c.Request.Body)

	hangoutsURL, err := url.Parse(c.Request.URL.Query().Get("url"))
	if err != nil {
		logrus.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	data := alertData{}
	err = dec.Decode(&data)
	data.QueryParams = c.Request.URL.Query()
	if err != nil {
		logrus.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	tmpl, err := generateTemplate(messageTemplate, data)
	if err != nil {
		logrus.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	textReq := &textRequest{
		Text: tmpl,
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	err = enc.Encode(&textReq)

	if err != nil {
		logrus.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	err = sendChatMessage(hangoutsURL, buf)
	if err != nil {
		logrus.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
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

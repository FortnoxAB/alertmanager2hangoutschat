# alertmanager2hangoutschat

alertmanager2hangoutschat takes a alertmanager webhook and sends the alert to hangouts chat.

Example alertmanager webhook config:
```
  webhook_configs:
  - url: http://alertmanager2hangoutschat.domain.com/api/alertmanager2hangoutschat/alertmanager?url=https%3A%2F%2Fchat.googleapis.com%2Fv1%2Fspaces%2Fasdfasdf%2Fmessages%3Fkey%3DKEY%26token%3DTOKEN

```

The endpoints takes the google webhook api as urlencoded GET parameter named "url". 


### build binary

```
go get -u github.com/fortnoxab/alertmanager2hangoutschat
```

### Usage
```
Usage of alertmanager2hangoutschat:
  -log-format string
    	can be empty string or json
  -log-level string
    	Can be one of:panic,fatal,error,warning,info,debug (default "info")
  -path string
    	What path to listen to for POST requests (default "/api/alertmanager2hangoutschat/alertmanager")
  -port string
    	Port to listen to (default "8080")
  -template-string string
    	template for the messages sent to hangouts chat (default "{{ define \"print_annotations\" }}{{ range . }}\n*{{ .Labels.alertname }}*\n{{ range .Annotations.SortedPairs -}}\n{{ .Name }}: {{ .Value}}\n{{ end -}}\nSource: <{{ .GeneratorURL }}|Show in prometheus>\n{{ end -}}{{ end -}}\n<users/all>\n[{{ .Status | toUpper }}{{ if eq .Status \"firing\" }}:{{ .Alerts.Firing | len }}{{ end }}]\n{{ if gt (len .Alerts.Firing) 0 -}}\n{{ template \"print_annotations\" .Alerts.Firing -}}\n{{ end -}}\n{{ if gt (len .Alerts.Resolved) 0 -}}\n{{ template \"print_annotations\" .Alerts.Resolved -}}\n{{ end -}}\n")
```

package utils

import (
	"encoding/json"
	"strings"

	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	"github.com/hashicorp/go-hclog"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubeConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, pluginTypes.RpcError{ErrorString: err.Error()}
	}
	return config, nil
}

func SetupLog(logFormat string) *log.Entry {
	logger := log.New()
	logger.SetLevel(log.InfoLevel)
	if strings.EqualFold(logFormat, "json") {
		logger.SetFormatter(newHCLogJSONFormatter())
	} else {
		logger.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}
	return logger.WithFields(log.Fields{"plugin": "trafficrouter"})
}

// hcLogJSONFormatter emits JSON log lines with field names and timestamps that
// hashicorp go-plugin understands when relaying plugin stderr to hclog.
type hcLogJSONFormatter struct {
	log.JSONFormatter
}

func newHCLogJSONFormatter() *hcLogJSONFormatter {
	return &hcLogJSONFormatter{
		JSONFormatter: log.JSONFormatter{
			TimestampFormat: hclog.TimeFormatJSON,
			FieldMap: log.FieldMap{
				log.FieldKeyTime:  "@timestamp",
				log.FieldKeyLevel: "@level",
				log.FieldKeyMsg:   "@message",
			},
		},
	}
}

func (f *hcLogJSONFormatter) Format(entry *log.Entry) ([]byte, error) {
	b, err := f.JSONFormatter.Format(entry)
	if err != nil {
		return nil, err
	}

	const levelKey = "@level"
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return b, nil
	}

	lvl, ok := raw[levelKey].(string)
	if !ok {
		return b, nil
	}

	normalized := normalizeHCLogLevel(lvl)
	if normalized == lvl {
		return b, nil
	}

	raw[levelKey] = normalized
	out, err := json.Marshal(raw)
	if err != nil {
		return b, nil
	}
	return append(out, '\n'), nil
}

func normalizeHCLogLevel(lvl string) string {
	switch lvl {
	case "warning":
		return "warn"
	case "panic", "fatal":
		return "error"
	default:
		return lvl
	}
}

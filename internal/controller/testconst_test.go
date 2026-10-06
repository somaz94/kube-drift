package controller

// Fixture values shared across the controller tests. Secret-contract key names
// live in driftcheck_controller.go; secretKeyURL is a fixture's own key.
const (
	nsDefault = "default"

	kindConfigMap = "ConfigMap"
	kindService   = "Service"

	nameDesired   = "desired"
	nameAppConfig = "app-config"

	secretNameCreds = "creds"
	secretKeyURL    = "url"

	testRepoURL    = "https://example.com/repo.git"
	testWebhookURL = "http://hook.example/1"
	testMissingKey = "nope"
)

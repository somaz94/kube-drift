package controller

// Fixture values shared across the controller tests. Kept in one place so a
// repeated namespace, kind, or Secret field reads the same in every test.
// Secret *key* names are not here — those live with the production code in
// driftcheck_controller.go, because they are part of the Secret contract.
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

package vo

type Platform string

const (
	PlatformCLI          Platform = "cli"
	PlatformTelegram     Platform = "telegram"
	PlatformAlertmanager Platform = "alertmanager"
	PlatformSlack        Platform = "slack"
	PlatformDiscord      Platform = "discord"
)

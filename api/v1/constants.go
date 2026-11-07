package v1

const (
	EnvPrefectAPIDatabaseDriver         = "PREFECT_API_DATABASE_DRIVER"
	EnvPrefectAPIDatabaseHost           = "PREFECT_API_DATABASE_HOST"
	EnvPrefectAPIDatabaseName           = "PREFECT_API_DATABASE_NAME"
	EnvPrefectAPIDatabaseMigrateOnStart = "PREFECT_API_DATABASE_MIGRATE_ON_START"
	EnvPrefectAPIDatabasePassword       = "PREFECT_API_DATABASE_PASSWORD"
	EnvPrefectAPIDatabasePort           = "PREFECT_API_DATABASE_PORT"
	EnvPrefectAPIDatabaseUser           = "PREFECT_API_DATABASE_USER"

	EnvPrefectHome                   = "PREFECT_HOME"
	EnvPrefectMessagingBroker        = "PREFECT_MESSAGING_BROKER"
	EnvPrefectMessagingCache         = "PREFECT_MESSAGING_CACHE"
	EnvPrefectRedisMessagingDB       = "PREFECT_REDIS_MESSAGING_DB"
	EnvPrefectRedisMessagingHost     = "PREFECT_REDIS_MESSAGING_HOST"
	EnvPrefectRedisMessagingPort     = "PREFECT_REDIS_MESSAGING_PORT"
	EnvPrefectRedisMessagingUsername = "PREFECT_REDIS_MESSAGING_USERNAME"
	EnvPrefectRedisMessagingPassword = "PREFECT_REDIS_MESSAGING_PASSWORD"

	EnvPrefectServerConcurrencyLeaseStorage = "PREFECT_SERVER_CONCURRENCY_LEASE_STORAGE"

	PrefectHomePath   = "/var/lib/prefect/"
	PrefectSQLitePath = "/var/lib/prefect/prefect.db"

	MigrateOnStartFalse = "False"

	ServerArgHost            = "--host"
	WorkerArgPool            = "--pool"
	WorkerArgType            = "--type"
	WorkerArgWithHealthcheck = "--with-healthcheck"

	RedisMessagingPackage    = "prefect_redis.messaging"
	RedisLeaseStoragePackage = "prefect_redis.lease_storage"

	PrefectCLI       = "prefect"
	WorkPoolTypeK8s  = "kubernetes"
	StartCommand     = "start"
	ServerSubcommand = "server"
	WorkerSubcommand = "worker"

	DatabaseDriverSQLite   = "sqlite+aiosqlite"
	DatabaseDriverPostgres = "postgresql+asyncpg"
	MigrateOnStartTrue     = "True"
	LabelPrefectIOServer   = "prefect.io/server"
	AppLabelPrefectServer  = "prefect-server"

	// Container names. They are stable, because pod templates match containers by name.
	ContainerNamePrefectServer   = AppLabelPrefectServer
	ContainerNameWaitForDatabase = "wait-for-database"
	ContainerNameMigration       = "prefect-server-migration"
	ContainerNameWorker          = "prefect-worker"
)

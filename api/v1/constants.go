/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
)

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

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("PrefectServer type", func() {
	It("can be deep copied", func() {
		original := &PrefectServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: PrefectServerSpec{
				Version: ptr.To("0.0.1"),
				Image:   ptr.To("prefecthq/prefect:0.0.1"),
				SQLite: &SQLiteConfiguration{
					StorageClassName: "standard",
					Size:             resource.MustParse("1Gi"),
				},
			},
		}

		copied := original.DeepCopy()

		Expect(copied).To(Equal(original))
		Expect(copied).NotTo(BeIdenticalTo(original))
	})

	Context("Server environment variables", func() {
		It("should generate correct environment variables for PostgreSQL with direct values", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Postgres: &PostgresConfiguration{
						Host:     ptr.To("postgres.example.com"),
						Port:     ptr.To(5432),
						User:     ptr.To("prefect"),
						Password: ptr.To("secret123"),
						Database: ptr.To("prefect"),
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "postgresql+asyncpg"},
				{Name: "PREFECT_API_DATABASE_HOST", Value: "postgres.example.com"},
				{Name: "PREFECT_API_DATABASE_PORT", Value: "5432"},
				{Name: "PREFECT_API_DATABASE_USER", Value: "prefect"},
				{Name: "PREFECT_API_DATABASE_PASSWORD", Value: "secret123"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "prefect"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "False"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should generate correct environment variables for PostgreSQL with environment variable sources", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Postgres: &PostgresConfiguration{
						HostFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
								Key:                  "host",
							},
						},
						PortFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
								Key:                  "port",
							},
						},
						UserFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
								Key:                  "username",
							},
						},
						PasswordFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
								Key:                  "password",
							},
						},
						DatabaseFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
								Key:                  "database",
							},
						},
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "postgresql+asyncpg"},
				{
					Name: "PREFECT_API_DATABASE_HOST",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
							Key:                  "host",
						},
					},
				},
				{
					Name: "PREFECT_API_DATABASE_PORT",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
							Key:                  "port",
						},
					},
				},
				{
					Name: "PREFECT_API_DATABASE_USER",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
							Key:                  "username",
						},
					},
				},
				{
					Name: "PREFECT_API_DATABASE_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
							Key:                  "password",
						},
					},
				},
				{
					Name: "PREFECT_API_DATABASE_NAME",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
							Key:                  "database",
						},
					},
				},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "False"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should generate correct environment variables for SQLite", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					SQLite: &SQLiteConfiguration{
						StorageClassName: "standard",
						Size:             resource.MustParse("1Gi"),
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should generate correct environment variables for ephemeral storage", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Ephemeral: &EphemeralConfiguration{},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should include additional settings in environment variables", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Ephemeral: &EphemeralConfiguration{},
					Settings: []corev1.EnvVar{
						{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
						{Name: "PREFECT_ANOTHER_SETTING", Value: "another-value"},
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
				{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
				{Name: "PREFECT_ANOTHER_SETTING", Value: "another-value"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should combine database and Redis configuration", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Postgres: &PostgresConfiguration{
						Host:     ptr.To("postgres.example.com"),
						Port:     ptr.To(5432),
						User:     ptr.To("prefect"),
						Password: ptr.To("secret123"),
						Database: ptr.To("prefect"),
					},
					Redis: &RedisConfiguration{
						Host:     ptr.To("redis.example.com"),
						Port:     ptr.To(6379),
						Database: ptr.To(0),
					},
					Settings: []corev1.EnvVar{
						{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				// Postgres vars
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "postgresql+asyncpg"},
				{Name: "PREFECT_API_DATABASE_HOST", Value: "postgres.example.com"},
				{Name: "PREFECT_API_DATABASE_PORT", Value: "5432"},
				{Name: "PREFECT_API_DATABASE_USER", Value: "prefect"},
				{Name: "PREFECT_API_DATABASE_PASSWORD", Value: "secret123"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "prefect"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "False"},
				// Redis vars
				{Name: "PREFECT_MESSAGING_BROKER", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_MESSAGING_CACHE", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_REDIS_MESSAGING_HOST", Value: "redis.example.com"},
				{Name: "PREFECT_REDIS_MESSAGING_PORT", Value: "6379"},
				{Name: "PREFECT_REDIS_MESSAGING_DB", Value: "0"},
				// Extra settings
				{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})
	})

	Context("Redis configuration", func() {
		It("should generate correct environment variables with direct values", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Redis: &RedisConfiguration{
						Host:     ptr.To("redis.example.com"),
						Port:     ptr.To(6379),
						Database: ptr.To(0),
						Username: ptr.To("prefect"),
						Password: ptr.To("secret123"),
					},
				},
			}

			envVars := server.Spec.Redis.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_MESSAGING_BROKER", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_MESSAGING_CACHE", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_REDIS_MESSAGING_HOST", Value: "redis.example.com"},
				{Name: "PREFECT_REDIS_MESSAGING_PORT", Value: "6379"},
				{Name: "PREFECT_REDIS_MESSAGING_DB", Value: "0"},
				{Name: "PREFECT_REDIS_MESSAGING_USERNAME", Value: "prefect"},
				{Name: "PREFECT_REDIS_MESSAGING_PASSWORD", Value: "secret123"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should generate correct environment variables with environment variable sources", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Redis: &RedisConfiguration{
						HostFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "redis-config"},
								Key:                  "host",
							},
						},
						PortFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "redis-config"},
								Key:                  "port",
							},
						},
						DatabaseFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "redis-config"},
								Key:                  "database",
							},
						},
						UsernameFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "redis-secret"},
								Key:                  "username",
							},
						},
						PasswordFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "redis-secret"},
								Key:                  "password",
							},
						},
					},
				},
			}

			envVars := server.Spec.Redis.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_MESSAGING_BROKER", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_MESSAGING_CACHE", Value: "prefect_redis.messaging"},
				{
					Name: "PREFECT_REDIS_MESSAGING_HOST",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "redis-config"},
							Key:                  "host",
						},
					},
				},
				{
					Name: "PREFECT_REDIS_MESSAGING_PORT",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "redis-config"},
							Key:                  "port",
						},
					},
				},
				{
					Name: "PREFECT_REDIS_MESSAGING_DB",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "redis-config"},
							Key:                  "database",
						},
					},
				},
				{
					Name: "PREFECT_REDIS_MESSAGING_USERNAME",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "redis-secret"},
							Key:                  "username",
						},
					},
				},
				{
					Name: "PREFECT_REDIS_MESSAGING_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "redis-secret"},
							Key:                  "password",
						},
					},
				},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should include Redis environment variables in server configuration", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Redis: &RedisConfiguration{
						Host:     ptr.To("redis.example.com"),
						Port:     ptr.To(6379),
						Database: ptr.To(0),
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedRedisEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_MESSAGING_BROKER", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_MESSAGING_CACHE", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_REDIS_MESSAGING_HOST", Value: "redis.example.com"},
				{Name: "PREFECT_REDIS_MESSAGING_PORT", Value: "6379"},
				{Name: "PREFECT_REDIS_MESSAGING_DB", Value: "0"},
			}

			for _, expected := range expectedRedisEnvVars {
				Expect(envVars).To(ContainElement(expected))
			}
		})

		It("should handle partial Redis configuration", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Redis: &RedisConfiguration{
						Host: ptr.To("redis.example.com"),
						// Only specifying host, other fields left empty
					},
				},
			}

			envVars := server.Spec.Redis.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_MESSAGING_BROKER", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_MESSAGING_CACHE", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_REDIS_MESSAGING_HOST", Value: "redis.example.com"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})
	})

	Context("Additional settings", func() {
		It("should include settings with direct values", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Ephemeral: &EphemeralConfiguration{},
					Settings: []corev1.EnvVar{
						{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
						{Name: "PREFECT_ANOTHER_SETTING", Value: "another-value"},
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
				{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
				{Name: "PREFECT_ANOTHER_SETTING", Value: "another-value"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should include settings with environment variable sources", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Ephemeral: &EphemeralConfiguration{},
					Settings: []corev1.EnvVar{
						{
							Name: "PREFECT_EXTRA_SETTING",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-config"},
									Key:                  "extra-setting",
								},
							},
						},
						{
							Name: "PREFECT_SECRET_SETTING",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-secret"},
									Key:                  "secret-setting",
								},
							},
						},
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
				{
					Name: "PREFECT_EXTRA_SETTING",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-config"},
							Key:                  "extra-setting",
						},
					},
				},
				{
					Name: "PREFECT_SECRET_SETTING",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-secret"},
							Key:                  "secret-setting",
						},
					},
				},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should merge settings with all configurations", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Postgres: &PostgresConfiguration{
						Host:     ptr.To("postgres.example.com"),
						Port:     ptr.To(5432),
						User:     ptr.To("prefect"),
						Password: ptr.To("secret123"),
						Database: ptr.To("prefect"),
					},
					Redis: &RedisConfiguration{
						Host:     ptr.To("redis.example.com"),
						Port:     ptr.To(6379),
						Database: ptr.To(0),
					},
					Settings: []corev1.EnvVar{
						{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
						{
							Name: "PREFECT_SECRET_SETTING",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-secret"},
									Key:                  "secret-setting",
								},
							},
						},
					},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				// Postgres vars
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "postgresql+asyncpg"},
				{Name: "PREFECT_API_DATABASE_HOST", Value: "postgres.example.com"},
				{Name: "PREFECT_API_DATABASE_PORT", Value: "5432"},
				{Name: "PREFECT_API_DATABASE_USER", Value: "prefect"},
				{Name: "PREFECT_API_DATABASE_PASSWORD", Value: "secret123"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "prefect"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "False"},
				// Redis vars
				{Name: "PREFECT_MESSAGING_BROKER", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_MESSAGING_CACHE", Value: "prefect_redis.messaging"},
				{Name: "PREFECT_REDIS_MESSAGING_HOST", Value: "redis.example.com"},
				{Name: "PREFECT_REDIS_MESSAGING_PORT", Value: "6379"},
				{Name: "PREFECT_REDIS_MESSAGING_DB", Value: "0"},
				// Extra settings
				{Name: "PREFECT_EXTRA_SETTING", Value: "extra-value"},
				{
					Name: "PREFECT_SECRET_SETTING",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-secret"},
							Key:                  "secret-setting",
						},
					},
				},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		It("should handle empty settings", func() {
			server := &PrefectServer{
				Spec: PrefectServerSpec{
					Ephemeral: &EphemeralConfiguration{},
					Settings:  []corev1.EnvVar{},
				},
			}

			envVars := server.ToEnvVars()

			expectedEnvVars := []corev1.EnvVar{
				{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
				{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
				{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
			}

			Expect(envVars).To(ConsistOf(expectedEnvVars))
		})

		Context("Host binding configuration", func() {
			It("should use default host 0.0.0.0 when Host is nil", func() {
				server := &PrefectServer{
					Spec: PrefectServerSpec{},
				}

				args := server.EntrypointArguments()
				Expect(args).To(Equal([]string{"prefect", "server", "start", "--host", "0.0.0.0"}))
			})

			It("should use empty string for IPv6/dual-stack when specified", func() {
				server := &PrefectServer{
					Spec: PrefectServerSpec{
						Host: ptr.To(""),
					},
				}

				args := server.EntrypointArguments()
				Expect(args).To(Equal([]string{"prefect", "server", "start", "--host", ""}))
			})

			It("should use custom host with ExtraArgs", func() {
				server := &PrefectServer{
					Spec: PrefectServerSpec{
						Host:      ptr.To(""),
						ExtraArgs: []string{"--some-arg", "some-value"},
					},
				}

				args := server.EntrypointArguments()
				Expect(args).To(Equal([]string{"prefect", "server", "start", "--host", "", "--some-arg", "some-value"}))
			})

			It("should use specific IPv4 address when specified", func() {
				server := &PrefectServer{
					Spec: PrefectServerSpec{
						Host: ptr.To("127.0.0.1"),
					},
				}

				args := server.EntrypointArguments()
				Expect(args).To(Equal([]string{"prefect", "server", "start", "--host", "127.0.0.1"}))
			})

			It("should maintain backward compatibility with deprecated EntrypointArugments method", func() {
				server := &PrefectServer{
					Spec: PrefectServerSpec{
						Host: ptr.To(""),
					},
				}

				// Test that the deprecated method still works
				args := server.EntrypointArugments()
				Expect(args).To(Equal([]string{"prefect", "server", "start", "--host", ""}))
			})
		})
	})
})

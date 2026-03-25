sed -i 's/if if assert.Len(t, jobs, 1) { {/if assert.Len(t, jobs, 1) {/' internal/orchestrator/api_generic_webhook_test.go
sed -i 's/		} \n		}/		}/' internal/orchestrator/api_generic_webhook_test.go

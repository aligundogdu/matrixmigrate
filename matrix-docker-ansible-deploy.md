# Additional Information relevant for users of [matrix-docker-ansible-deploy](https://github.com/spantaleev/matrix-docker-ansible-deploy)

first follow the [quick-start.md](https://github.com/spantaleev/matrix-docker-ansible-deploy/blob/master/docs/quick-start.md)

## In Inventory/host_vars/your-host/vars.yml set:

```
# debug logs
matrix_synapse_log_level: "INFO"
matrix_synapse_storage_sql_log_level: "INFO"
matrix_synapse_root_log_level: "INFO"
```
- to see the logs use: journalctl -fu matrix-synapse

## In roles/custom/matrix-synapse/templates/synapse/homeserver.yaml.j2

- to disable rate limiting set

```
rc_room_creation:
  per_second: 10000
  burst_count: 10000
```

## In roles/custom/matrix-synapse/defaults/main.yml set:

- "matrix_synapse_container_client_api_host_bind_port:" to a port that you configure in the config.yml of matrixmigrate

- create a volume for the matrixmigrate.yaml

```
matrix_synapse_container_additional_volumes:
- src: /opt/matrix/matrixmigrate.yaml
  dst: /matrixmigrate.yaml
  options: ro
```

- mount it 
```
matrix_synapse_app_service_config_files:
- /matrixmigrate.yaml
```

- disable rate limits
```
matrix_synapse_rc_message:
  per_second: 10000
  burst_count: 10000

matrix_synapse_rc_registration:
  per_second: 10000
  burst_count: 10000

matrix_synapse_rc_login:
  address:
    per_second: 10000
    burst_count: 10000
  account:
    per_second: 10000
    burst_count: 10000
  failed_attempts:
    per_second: 10000
    burst_count: 10000

matrix_synapse_rc_admin_redaction:
  per_second: 10000
  burst_count: 10000

matrix_synapse_rc_joins:
  local:
    per_second: 10000
    burst_count: 10000
  remote:
    per_second: 10000
    burst_count: 10000


matrix_synapse_rc_invites:
  per_room:
    per_second: 10000
    burst_count: 10000
  per_user:
    per_second: 10000
    burst_count: 10000
  per_issuer:
    per_second: 10000
    burst_count: 10000
```
- set "matrix_synapse_auto_accept_invites_enabled: true" as described in the README

- apply changes using: ansible-playbook -i inventory/hosts setup.yml --tags=setup-all,start

## Create the /opt/matrix/matrixmigrate.yaml file with the contents according to the README

- to restart the docker restart the matrix-synapse systemd service



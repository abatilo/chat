# chat

[![Go Report Card](https://goreportcard.com/badge/github.com/abatilo/chat)](https://goreportcard.com/report/github.com/abatilo/chat)

The following is a repository for a simple chat application architecture.

The application is currently hosted at https://chat.aaronbatilo.dev

## Fun things I did

In no particular order, here are a few things in this project that I'm excited about and want to highlight.

- [x] [README is largely generated](./scripts/update_readme.sh) and is [updated automatically by a GitHub Action](./.github/workflows/update-readme.yml)
- [x] Database model makes it easy to add new message types as we [leverage Postgres JSON columns](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/internal/cmd/api/routes.go#L309-L312) to shape the data for HTTP response
- [x] [bcrypt](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/internal/cmd/api/routes.go#L80) hashed and salted password storage
- [x] [Session management with expiration](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/internal/cmd/api/cmd.go#L50-L53) for tracking logins and API tokens
- [x] Compilation is done to a [single statically linked binary](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/Dockerfile#L14) making for easy deployment
- [x] Configuration follows [12Factor](https://12factor.net/config) best practices and allows for [environment based configuration](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/cmd/chat.go#L18-L24)
- [x] [End to end integration tests](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/.github/workflows/pr-integration-tests.yml#L26-L39) are ran in a self contained environment on every PR
- [x] Zero down time deployments are done with a combination of:
  - [Readiness check](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/deployments/pulumi/index.ts#L54-L56)
  - [Liveness check](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/deployments/pulumi/index.ts#L57-L60)
  - [Pod disruption budget](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/deployments/pulumi/index.ts#L84-L92)
  - [preStop hook](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/deployments/pulumi/index.ts#L62-L66)
  - [Signal capturing with request draining](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/internal/cmd/api/cmd.go#L63-L78)
- [x] [pprof endpoints](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/internal/cmd/api/server.go#L145-L150) for profiling information
- [x] [Prometheus](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/internal/cmd/api/server.go#L143) metrics [exposed](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/deployments/pulumi/index.ts#L123-L130) for tracking latency and request rates for every endpoint. [Example](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/internal/cmd/api/routes.go#L273-L276)
- [x] [Deployed on every push](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/.github/workflows/pulumi.yml#L46-L56). Hosted and [available](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/deployments/pulumi/index.ts#L115-L122) at https://chat.aaronbatilo.dev
- [x] [Network level rate limiting](https://github.com/abatilo/chat/blob/1d1535e3eca8365555884775411f3a311153948d/deployments/pulumi/index.ts#L99-L101) for security and availability
- [ ] Multi-region deployment following data residency for users
- [ ] Granular unit tests. There _were_ some that had [about 50% coverage](https://github.com/abatilo/chat/commit/71836dacf5e3641113c33da5f81b776ad381c154) but I opted to remove them in favor of the end to end integration tests.

## Getting started

Local development environment is managed largely with
[asdf-vm](https://asdf-vm.com/). Install `asdf` to make it easier to manage all
of your tools. Run `./asdf.sh` to install all the versions and plugins you need.

<!-- BEGIN_TOOL_VERSIONS -->

```
⇒ cat .tool-versions
golang 1.16.7
syncher 0.0.44
chamber 2.10.1
tilt 0.22.3
gomigrate 4.14.1
pulumi 3.9.1
nodejs 14.17.4
```

<!-- END_TOOL_VERSIONS -->

## Routes

Application business logic is implemented in
[./internal/cmd/api/routes.go](./internal/cmd/api/routes.go) and follows a
simple setup.

<!-- BEGIN_REGISTER_ROUTES -->

```golang
func (s *Server) registerRoutes() {
	// Register session middleware
	s.router.Use(s.sessionManager.LoadAndSave)

	// Application routes
	s.router.Get("/check", s.ping())
	s.router.Post("/users", s.createUser())
	s.router.Post("/login", s.login())
	s.router.Route("/messages", func(r chi.Router) {
		r.Use(s.authRequired())
		r.Post("/", s.createMessage())
		r.Get("/", s.listMessages())
	})
}
```

<!-- END_REGISTER_ROUTES -->

## Integration testing script

The following script demonstrates hitting all of the endpoints and message
types with happy path examples.

<!-- BEGIN_INTEGRATION_TEST -->

`⇒ cat ./scripts/integration_test.sh`

```bash
#!/bin/bash
set -e

echo "Starting integration tests"

host="${HOST:-https://chat.aaronbatilo.dev}"

username=$(openssl rand -base64 12)
password=$(openssl rand -base64 12)

echo "Creating a user..."
user_id=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/users" | jq -r '.id')
echo "Created user ${user_id}"

echo "Logging in to get a valid session token..."
token=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/login" | jq -r '.token')
echo "Login was successful. We can send requests with ${token}"


for i in {1..10}
do
  message_type=$(echo $(( $RANDOM % 3 )))
  if [ "${message_type}" == "0" ]; then
    echo "Creating text message..."
    text=$(openssl rand -base64 12)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"text\",\"text\":\"${text}\"}}" "${host}/messages"
  fi

  if [ "${message_type}" == "1" ]; then
    echo "Creating image message..."
    url=$(openssl rand -base64 12)
    width=$(echo $(( $RANDOM % 99 + 1 )))
    height=$(echo $(( $RANDOM % 99 + 1 )))
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"image\",\"url\":\"${url}\", \"width\": ${width}, \"height\": ${height}}}" "${host}/messages"
  fi

  if [ "${message_type}" == "2" ]; then
    echo "Create video message..."
    url=$(openssl rand -base64 12)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"video\",\"url\":\"${url}\", \"source\": \"youtube\"}}" "${host}/messages"
  fi
done

start=$(echo $(( $RANDOM % 500 + 1 )))
curl -s -X GET -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"recipient\": 1, \"start\": ${start}, \"limit\": 100}" "${host}/messages" | jq -c '.messages[]'
```

<!-- END_INTEGRATION_TEST -->

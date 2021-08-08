# chat

[![Go Report Card](https://goreportcard.com/badge/github.com/abatilo/chat)](https://goreportcard.com/report/github.com/abatilo/chat)

The following is a repository for a simple chat application architecture.

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

host="https://chat.aaronbatilo.dev"

username=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
password=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)

echo "Creating a user..."
user_id=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/users" | jq -r '.id')
echo "Created user ${user_id}"

echo "Logging in to get a valid session token..."
token=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/login" | jq -r '.token')
echo "Login was successful. We can send requests with ${token}"


for i in {1..10}
do
  message_type=$(cat /dev/urandom | tr -dc '0-2' | fold -w 512 | head -n 1 | head --bytes 1)
  if [ "${message_type}" == "0" ]; then
    echo "Creating text message..."
    text=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"text\",\"text\":\"${text}\"}}" "${host}/messages"
  fi

  if [ "${message_type}" == "1" ]; then
    echo "Creating image message..."
    url=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    width=$(cat /dev/urandom | tr -dc '0-9' | fold -w 256 | head -n 1 | sed -e 's/^0*//' | head --bytes 3)
    height=$(cat /dev/urandom | tr -dc '0-9' | fold -w 256 | head -n 1 | sed -e 's/^0*//' | head --bytes 3)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"image\",\"url\":\"${url}\", \"width\": ${width}, \"height\": ${height}}}" "${host}/messages"
  fi

  if [ "${message_type}" == "2" ]; then
    echo "Create video message..."
    url=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"video\",\"url\":\"${url}\", \"source\": \"youtube\"}}" "${host}/messages"
  fi
done

start=$(cat /dev/urandom | tr -dc '0-9' | fold -w 256 | head -n 1 | sed -e 's/^0*//' | head --bytes 2)
curl -s -X GET -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"recipient\": 1, \"start\": ${start}, \"limit\": 100}" "${host}/messages" | jq -c '.messages[]'
```

<!-- END_INTEGRATION_TEST -->

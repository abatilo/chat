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

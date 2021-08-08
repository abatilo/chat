#!/bin/sh
set -ex

awk '
    BEGIN                             {p=1}
    /^<!-- BEGIN_TOOL_VERSIONS -->$/  {print;print "";print "```";print "⇒ cat .tool-versions";system("cat .tool-versions");p=0}
    /^<!-- END_TOOL_VERSIONS -->$/    {print "```";print "";p=1}
    p' README.md > README.md.updated
mv README.md.updated README.md

routes="$(awk '
    BEGIN                             {p=0}
    /^\/\/ BEGIN registerRoutes$/     {p=1;next}
    /^\/\/ END registerRoutes$/       {p=0}
    p' internal/cmd/api/routes.go)"

awk -v routes="$routes" '
    BEGIN                               {p=1}
    /^<!-- BEGIN_REGISTER_ROUTES -->$/  {print;print "";printf "```golang";print routes;p=0}
    /^<!-- END_REGISTER_ROUTES -->$/    {print "```";print "";p=1}
    p' README.md > README.md.updated
mv README.md.updated README.md

awk '
    BEGIN                                 {p=1}
    /^<!-- BEGIN_INTEGRATION_TEST -->$/  {print;print "";print "`⇒ cat ./scripts/integration_test.sh`";print "";print "```bash";system("cat ./scripts/integration_test.sh");p=0}
    /^<!-- END_INTEGRATION_TEST -->$/    {print "```";print "";p=1}
    p' README.md > README.md.updated
mv README.md.updated README.md

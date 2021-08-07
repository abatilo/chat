#!/bin/sh
set -ex

awk '
    BEGIN                             {p=1}
    /^<!-- BEGIN_TOOL_VERSIONS -->$/  {print;print "";print "```";print "⇒ cat .tool-versions";system("cat .tool-versions");p=0}
    /^<!-- END_TOOL_VERSIONS -->$/    {print "```";print "";p=1}
    p' README.md > README.md.updated
mv README.md.updated README.md

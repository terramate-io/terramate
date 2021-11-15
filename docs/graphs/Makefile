all: depgraph.svg failure-mode-example.svg failure-mode-stack-a-fails.svg failure-mode-stack-b-fails.svg

%.svg: %.dot
	dot -Tsvg -Ecolor=red -o "$@" "$<"

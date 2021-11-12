all: depgraph.png failure-mode-example.png failure-mode-stack-a-fails.png

%.png: %.dot
	dot -Tpng -Ecolor=red -Earrowhead=diamond -o "$@" "$<"

all: depgraph.png failure-mode-example.png

%.png: %.dot
	dot -Tpng -Ecolor=red -Earrowhead=diamond -o "$@" "$<"

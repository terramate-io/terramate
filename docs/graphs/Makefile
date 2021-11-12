all: depgraph.png

%.png: %.dot
	dot -Tpng -Ecolor=red -Earrowhead=diamond -o "$@" "$<"

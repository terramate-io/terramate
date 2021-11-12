all: depgraph.png

depgraph.png:
	dot -Tpng -Ecolor=red -Earrowhead=diamond -o depgraph.png depgraph.dot

.PHONY: depgraph.png/view
depgraph.png/view: depgraph.png
	display depgraph.png

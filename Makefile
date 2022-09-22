.PHONY: default
default: help

ifeq ($(OS),Windows_NT)
-include makefiles/windows/*.mk
else 
-include makefiles/unix/*.mk
endif

-include makefiles/*.mk

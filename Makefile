SUBDIRS := $(wildcard *-service/.)

build: $(SUBDIRS)
$(SUBDIRS):
	echo "\nBuild $@"
	$(MAKE) -C $@

docker:
	docker-compose up --build

all: build docker


.PHONY: build dockers $(SUBDIRS)

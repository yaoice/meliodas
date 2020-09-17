# ==============================================================================
# Help

define HELPTEXT
Usage: make <OPTIONS> ... <TARGETS>

Targets:
    build              Build source code for host platform.
    build.all          Build source code for all platforms.
                       Best done in the cross build container
                       due to cross compiler dependencies.
    image              Build docker images and push to registry.
	deploy             Deploy updated components to development env.
	deploy.all         Deploy all components to development env.
    lint               Check syntax and styling of go sources.
    test               Run unit test.
    clean              Remove all files that are created by building.
    help               Show this help info.

Options:
    DEBUG        Whether to generate debug symbols. Default is 0.
    IMAGES       Backend images to make. All by default.
    PLATFORMS    The platform to build. Default is host platform and arch.
    BINS         The binaries to build. Default is all of cmd.
    VERSION      The version information compiled into binaries.
                 The default is obtained from git.
    V            Set to 1 enable verbose build. Default is 0.

endef
export HELPTEXT
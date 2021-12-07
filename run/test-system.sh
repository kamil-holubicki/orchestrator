#!/bin/bash

export TARBALL_URL=
export RUN_TESTS=
export ALLOW_TESTS_FAILURES=

export TARBALL_URL=https://downloads.percona.com/downloads/Percona-Server-8.0/Percona-Server-8.0.26-16/binary/tarball/Percona-Server-8.0.26-16-Linux.x86_64.glibc2.12-minimal.tar.gz
export RUN_TESTS=YES
export ALLOW_TESTS_FAILURES=YES

script/dock system

---
weight: 90
toc: true
title: Contributing
summary: ""
seo:
    canonical: ""
    description: ""
    robots: ""
    title: Contributing to Thanos Operator
lastmod: "2023-09-07T16:04:48+02:00"
draft: false
description: Guide to contributing to the Thanos Operator project
date: "2023-09-07T16:04:48+02:00"
---

This document explains the process of contributing to the Thanos Operator project.

First of all please follow the [Code of Conduct](../thanos-community-code-of-conduct) in all your interactions within the project.

## Thanos Philosophy

The philosophy of Thanos and our community borrows heavily from UNIX philosophy and the Golang programming language.

* Each subcommand should do one thing and do it well.
  * eg. Thanos query proxies incoming calls to known store API endpoints merging the result
* Write components that work together.
  * e.g. blocks should be stored in native Prometheus format
* Make it easy to read, write, and run components.
  * e.g. reduce complexity in system design and implementation

## Feedback / Issues

If you encounter any issue or you have an idea to improve, please:

* Search through Google and [existing open and closed GitHub Issues](https://github.com/thanos-community/thanos-operator/issues) for the answer first. If you find a relevant topic, please comment on the issue.
* If none of the issues are relevant, please add an issue to [GitHub issues](https://github.com/thanos-community/thanos-operator/issues). Please provide any relevant information as suggested by the Issue template.
* If you have a quick question you might want to also ask on #thanos or #thanos-operator slack channel in the CNCF workspace. We recommend using GitHub issues for issues and feedback, because GitHub issues are trackable.

If you encounter a security vulnerability, please refer to [Reporting a Vulnerability process](https://github.com/thanos-community/thanos-operator/blob/main/SECURITY.md#reporting-a-vulnerability)

## Adding New Features / Components

When contributing a complex change to Thanos Operator repository, please discuss the change you wish to make within a Github issue, in Slack, or by another method with the owners of this repository before making the change.

## General Naming

In the code and documentation prefer non-offensive terminology, for example:

* `allowlist` / `denylist` (instead of `whitelist` / `blacklist`)
* `primary` / `replica` (instead of `master` / `slave`)
* `openbox` / `closedbox` (instead of `whitebox` / `blackbox`)

## Components/CRDs Naming Architecture

Please follow the upstream conventions used by Thanos and Prometheus Operator projects, when deciding on new names for this project.

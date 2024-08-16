# lldpNeighbor

# LLDP Analysis Program Documentation

## Introduction

This program is designed to analyze the `show lldp` command across a set of network devices within a fabric. The analysis results will be stored in an Excel file (`example.xlsx`) for easy reference and further analysis.

## Configuration

Before running the program, you need to configure the connection parameters and target devices in a configuration file named `config.ini`.

### `config.ini` File

Below is an example of the `config.ini` file that you should create and configure according to your needs:

```ini
[global]
username = admin
password = admin
devices = Spine1,Spine2,Spine3,Leaf1a

[transport]
transport = https
port = 443

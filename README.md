# Sinking Yachts API Client

This is an unofficial API client for the Sinking Yachts API written in golang.

## Client Interfaces

This library provides 2 different interfaces for interacting with the API.

## Client

Client is high level interface, all domains will be cached locally.

It provides simple means to listen to the websocket api and get realtime updates.

`AutoSync` will allow you to listen to the websocket api and periodical full sync.

## RawClient

RawClient provides low level access to the API, all methods calls the api directly.
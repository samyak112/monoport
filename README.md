## Monoport:

This is a minimal Selective Forwarding Unit (SFU) implementation built in Go, along with a [React-based frontend](https://github.com/samyak112/streamio-frontend), aimed at exploring a key networking challenge in WebRTC: media flow through symmetric NATs. 

**This project works locally, and the next step is to deploy it and test its behavior in real-world NAT environments.**

**Try running the experiment [here](https://6847fa4070e65a4b9c94bdb5--symphonious-frangollo-9d1ad5.netlify.app/stream)**

**Update** - Its working with symmetric NAT need to do more tests

## Problem:
Most WebRTC systems rely on ICE (Interactive Connectivity Establishment), which uses STUN to discover the public-facing IP and port of a peer behind a NAT, and uses TURN as a fallback relay when direct peer-to-peer connections fail.

The issue? Symmetric NATs.

In symmetric NATs, the NAT maps internal IP:port combinations to unique external IP:port combinations per destination. So even if a STUN server tells you your public IP:port, it's only valid for communicating with that STUN server, not for the SFU or another peer.

## Experiment:
If the STUN server and SFU live on the same UDP port, the NAT may treat them as the same destination, and not remap the IP:port. That might let us bypass symmetric NATs, because the STUN-determined IP:port would be valid for media too.

This project is an experimental validation of this hypothesis.

## Features
1. Accepts WebRTC media via a shared UDP port (same as STUN).
2. Handles signaling over WebSockets.
3. Forwards media tracks to multiple peers.
4. Internally integrates a basic STUN handler.
5. Queue management of multiple offers negotiation from same Peer

## Why this could be cool?
Symmetric NATs are really common in modern networks (especially in mobile ISPs and carrier-grade NATs). They severely hinder peer-to-peer media delivery. TURN is the standard workaround, but:

TURN is expensive and introduces latency.

TURN servers require public IPs and significant infrastructure.

Bypassing symmetric NATs without TURN could reduce cost and complexity in edge-based systems.

If this idea works, SFU deployments could be made simpler and more reliable, especially in cases where TURN is infeasible or undesired.

**Now am sure people have already tried this out but I didnt found much data around this idea so I tried to implement it myself.**

## Limitations

While making this project I understood one thing that we cannot 100% remove the need of TURN servers because , sometimes peers are behind NATs which are super strict
and wont even allow UDP connection or some other sort of strict measures which are taken by Universities or Corporates so yes we cant totally remove the need of TURN servers
but maybe we can reduce the need?

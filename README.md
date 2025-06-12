## Monoport:

This is a minimal Selective Forwarding Unit (SFU) implementation built in Go, along with a [React-based frontend](https://github.com/samyak112/streamio-frontend), aimed at exploring a key networking challenge in WebRTC: media flow through symmetric NATs. 

**Try running the experiment [here](https://684ac13d0549b02e98b3e0a4--symphonious-frangollo-9d1ad5.netlify.app/stream)**

**Update** - Its working with symmetric NAT need to do more tests

## Problem:
Most WebRTC systems rely on ICE (Interactive Connectivity Establishment), which uses STUN to discover the public-facing IP and port of a peer behind a NAT, and uses TURN as a fallback relay when direct peer-to-peer connections fail.

The issue? Symmetric NATs.

In symmetric NATs, the NAT maps internal IP:port combinations to unique external IP:port combinations per destination. So even if a STUN server tells you your public IP:port, it's only valid for communicating with that STUN server, not for the SFU or another peer.

## Initial Experiment:
If the STUN server and SFU live on the same UDP port, the NAT may treat them as the same destination, and not remap the IP:port. That might let us bypass symmetric NATs, because the STUN-determined IP:port would be valid for media too.

### Result

**TL;DR - It worked for Port Restricted Cone NAT but failed for Symmetric NAT**

After building the initial version of my SFU + STUN multiplexer, I ran into a critical limitation: symmetric NATs weren’t letting media through, even though I was doing everything “right.”

The core issue was **port randomization**. In symmetric NATs, the client’s public IP:port isn't stable across different destinations. When I tried to reuse the same UDP port for both STUN and SFU, I assumed the NAT would treat the traffic as coming from the same peer — but it didn’t.

Even though I was reusing the public IP and port observed during STUN gathering, the media packets were still getting dropped. This happened because the symmetric NAT had created a separate port mapping specifically for the STUN exchange. When the SFU later tried to send ICE connectivity checks or media to that same IP:port (originally opened just for STUN), the NAT forwarded it to the wrong internal destination — where nothing was listening. As a result, those packets were silently dropped.

## Second Experiment

This project is an experimental validation of this hypothesis. 
Instead of trying to guess or reuse ports, I flipped the strategy:
Let the frontend initiate a connectivity check using just fake local candidates — and watch which public port the NAT actually uses for that attempt.

Once that STUN packet reached me, I could read the true, working public IP:port and send it back to the client as a usable ICE candidate. That way, media would flow correctly, and TURN wouldn’t be needed.

### How it works

- **Initial connection** : The client connects to the server via WebSocket and sends a join-room message. The server maps the client's unique peerID to the instance of this web socket connection.

- **Client-Side Configuration**: The RTCPeerConnection on the frontend is initialized with an empty iceServers array (iceServers: []). This deliberately prevents the browser from discovering its own public IP via STUN.

- **Offer with Local Candidates Only**: When ICE gathering starts, the browser only finds local host candidates (e.g., 192.168.1.100). It sends its SDP offer over the WebSocket containing only these local candidates.

- **Server-Side Setup**: The server receives the offer and creates a Pion PeerConnection, which generates a unique ICE `ufrag` which is craeted by Pion (Lib for webrtc) for uniquely identifying a Peer connection, this how it identifies to which peerConnection an incoming packet belongs to. It maps this `ufrag` to the client's Peer ID which contains the websocket instance and make another hashmap where key is `ufrag` and value is the same Websocket connection reference to identify it later.

- **Server Candidate Signaling**: The server sends its own public ICE candidates to the client over the WebSocket.

- **NAT Traversal Initiated by the Client**: The client's browser now has the server's public candidates and begins sending STUN Binding Requests. As these packets traverse the client's symmetric NAT, the NAT establishes a stable public port mapping for this specific client-to-server communication path.

- **Packet Sniffing & Real-time Discovery**: The server's UDP listener receives the STUN request and inspects it to extract the client’s actual public IP and port (from the UDP packet's source) and the `ufrag` (from the STUN payload).

- **Injecting the "Magic" Candidate**: When backend recieves the connectivity packet it contains the same `ufrag` that was generated earlier , I extract that from the packet and map it in my hashmap, Now i have the client Address and Port which is specifically opened for ice connectivity and later for relay of media, the server identifies the correct client and constructs a new "server-reflexive" ICE candidate with the discovered IP and port. It "trickles" this new, valid candidate back to the client via WebSocket which we get using `ufrag` lookup.

- **Connection Established**: The client's browser adds the new candidate. And send it to backend to use this new candidate as well for connectivity check and because this path is already open on the NAT, the connection succeeds, and media begins to flow reliably. So the initial local hosts were just fake candidates to get some information about my NAT.

### Result
**I was able to bypass Symmetric NATs**

## Why this could be cool?

Symmetric NATs are common, especially in mobile networks and large ISPs. They are a major hurdle for peer-to-peer and SFU-based WebRTC systems. The standard solution is a TURN server, but:

- TURN is expensive, requiring significant bandwidth and infrastructure.
- TURN adds latency by relaying all media.
This method could reduce the cost and complexity of WebRTC deployments by minimizing the reliance on TURN servers.

**Now I'm sure others have explored this, but I couldn't find much public data on this specific approach, so I decided to build and document it myself. Please let me know if it already exists**

## Limitations

While making this project I understood one thing that we cannot 100% remove the need of TURN servers because , some highly restrictive corporate or university networks may block UDP traffic altogether. In these scenarios, a TURN server (especially over TCP/TLS) remains a necessary fallback. However, for a large percentage of users, this approach can reduce the need for TURN, not remove it completely maybe?

## Comma Compliance Signal Capture Policy Statement / Use Policy

### Why This Exists

This project provides an open-source tool for capturing Signal messages in a secure, transparent, and privacy-conscious manner, designed for users or organizations who explicitly choose to retain their own communication history for compliance, legal, or organizational reasons.  
We respect that Signal was built for private, non-retained communication and not for workplace use, but the reality is that people still use it in business settings. This tool is for those situations where archiving is necessary and everyone involved understands what that means.

### Respect for Signal’s Privacy Model

Signal was built to protect your privacy, and we respect that. This capture tool:

- **Does not interfere** with Signal’s core infrastructure, encryption, or protocols.
- **Does not weaken or bypass** end-to-end encryption.
- Only works if the device owner gives permission
- Operates **entirely outside of the Signal app** and ecosystem.

We are not affiliated with Signal and do not represent this tool as an official extension of the Signal platform.  
This tool does not modify Signal clients, servers, protocols, or end‑to‑end encryption. It operates solely on data the authorized device owner can already access.

### User Consent & Control

This tool is designed with user consent and data ownership as core principles:

- Only the device owner or an authorized user can initiate or configure archiving.
- Data access occurs through clearly defined, user-enabled methods.
- While the tool supports automatic uploads for regulatory compliance, data is only sent to storage destinations that have been approved by the user or established by their organization.
- No data is transmitted to external services outside of those intended and authorized storage endpoints.

Users and organizations are fully responsible for how they use this tool, including where archived data is stored, how it’s protected, and whether it is shared.  

Use of this tool may violate Signal’s Terms of Service if it is used to capture conversations without proper authority or consent. **You are solely responsible for confirming legal and contractual eligibility.**

### Transparency & Auditability

This project is fully open-source and licensed under the GPLv3, which means anyone can read the code, review its functionality, and suggest improvements.  
Contributions are welcome, especially those that enhance **security, auditability**, and user **access control.**  
We encourage independent audits and feedback to ensure the tool aligns with the privacy and security expectations of both users and the broader community.

### Disclaimers

- The tool is intended for transparent, authorized use, not for surveillance or covert monitoring.  
- It is designed for **regulated or informed use** in professional settings where communication retention is necessary and explicitly understood by all parties.  
- Using this tool may violate the terms of use of Signal if used **without consent or improperly**. Users assume full responsibility.

### Our Commitment

We believe people should be able to manage their own data **without giving up their privacy**. Our goal is to make useful, ethical tools for organizations that need to follow rules — without betraying the trust and values that Signal stands for.  
We believe users should have agency over their data without compromising privacy. Our goal is to provide transparent, ethical tools that empower organizations to meet legitimate compliance requirements while still respecting the privacy values that Signal represents.

## Key Features

- User specific Docker containers with complete isolation
- Real time encrypted messages streamig
- Scalable batch processing

and [many more](https://bbernhard.github.io/signal-cli-rest-api/)

---

## Quick Start


1. Start a container

```bash
$ docker compose up --build
```
### Configurations
Please setup docker environment variables

```env
JOB_ID=unique-session-identifier
WEBSOCKET_URL=ws://your-app:3000/cable?token=your-token
# Cryptographic Keys
SIGNAL_PRIVATE_KEY=your-app-private-key
SIGNAL_PUBLIC_KEY=your-signal-public-key
RAILS_PUBLIC_KEY=your-app-public-key

# Webhook Delivery
WEBHOOK_URL=http://your-app/whatsapp_webhooks/

# Batch Processing
BATCH_SIZE=50
```

## Documentation & Usage

### API Reference

The Swagger API documentation can be found [here](https://bbernhard.github.io/signal-cli-rest-api/). If you prefer a simple text file based API documentation have a look [here](https://github.com/bbernhard/signal-cli-rest-api/blob/master/doc/EXAMPLES.md).

## Architecture Diagram

```
                                    ┌─────────────────────────────────────────────┐
                                    │               CI/CD Pipeline                │
                                    │          GitHub Actions                     │
                                    └──────────────────┬──────────────────────────┘
                                                       │ Auto Deploy
                                                       ▼
                                         ┌─────────────────────────────────────────────┐
                                         │             Docker Container (Go)           │
                                         │  ┌─────────────────────────────────────┐    │
                                         │  │         signal-cli                  │    │
                                         │  │    (Signal Protocol API)            │    │
                                         │  └─────────────────────────────────────┘    │
┌─────────────────┐                      │  ┌─────────────────────────────────────┐    │
│      Signal     │◄────────────────────►│  │         Crypto Layer                │    │
│   Mobile App    │     Device Link      │  │  Signal Protocol + Double Ratchet   │    │
└─────────────────┘                      │  │  XChaCha20 + Ed25519 + Curve25519   │    │
         ▲                               │  └─────────────────────────────────────┘    │
         │                               └─────────────────┬───────────────────────────┘
         │ Messages                                        │
         ▼                                 ┌─────────────┼─────────────┐
                                           │             │             │
                                           ▼             ▼             ▼
                               ┌──────────────────┐  ┌──────────────────┐
                               │  WebSocket       │  │    Webhook       │
                               │  Real-time       │  │   HTTP POST      │
                               │  Streaming       │  │   Delivery       │
                               └─────────┬────────┘  └─────────┬────────┘
                                         │                     │
                                         └─────────┬───────────┘
                                                   │
                                                   ▼
                                         ┌─────────────────────┐
                                         │   Your App          │
                                         │   Backend           │
                                         │                     │
                                         │ • Message Processing│
                                         │ • Business Logic    │
                                         │ • User Management   │
                                         │ • Contact Sync      │
                                         └─────────────────────┘

```
The connector operates as a secure bridge between Signal CLI and your enterprise infrastructure, ensuring message delivery through multiple channels with full encryption support.

---
## Security & Compliance

**Cryptography**: Built with modern NaCl/libsodium encryption standards
- **Encryption**: XChaCha20 symmetric encryption
- **Key Exchange**: Curve25519 elliptic curve
- **Signatures**: Ed25519 digital signatures

## Roadmap
**FIPS Compliance**: A --fips flag is coming soon for FIPS supported encryption protocols but our current crypto stack is already safer. Unlike FIPS, we use modern, misuse-resistant algorithms like XChaCha20 and Ed25519 that offer better real-world security.

---

## Deployment

### CI/CD Pipeline
Automated builds are triggered on changes to `whatsapp-client/` directory:
- **Workflow**: `.github/workflows/ci-whatsapp-client.yml`
- **Registry**: GitHub Container Registry (GHCR)


## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

---

## Security & Bug Bounty <!-- TBD -->

We take security seriously. If you discover a security vulnerability, please:

1. **Do not** open a public issue
2. **Email** us at security@commacompliance.com
3. **Include** detailed steps to reproduce
4. **Wait** for our response before public disclosure

**Bug Bounty Program:** Coming soon — report vulnerabilities responsibly and earn rewards. Minimum bounty: $25 for valid submissions.

---

## Support

- **Enterprise Support**: contact@commacompliance.com

---
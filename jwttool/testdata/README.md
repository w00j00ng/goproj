# Published JWT test vectors

The HS256 compact token and HMAC key are from [RFC 7515 Appendix A.1](https://www.rfc-editor.org/rfc/rfc7515#appendix-A.1). The ES256 compact token and P-256 JWK are from [RFC 7515 Appendix A.3](https://www.rfc-editor.org/rfc/rfc7515#appendix-A.3). Display line wrapping was removed; signed bytes were preserved. Tests convert the published JWK to SEC1/PKCS8 private PEM and PKIX public PEM with Go's standard library. These are public example keys, never production credentials.

Both published tokens contain `exp:1300819380`; tests use the fixed historical time `1300819379`. Neither fixture was signed by this implementation.

RFC 7515 Copyright (c) 2015 IETF Trust and the persons identified as the document authors. All rights reserved. Code Components are provided under the Simplified BSD License described by the [IETF Trust Legal Provisions](https://trustee.ietf.org/trust-legal-provisions.html), without warranty.

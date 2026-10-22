# Dependency security patches

These pnpm patches mitigate two advisories for which the registry had no published fixed version on 2026-10-21. They do not suppress audit results or change package versions. The workspace maps exact package versions to the patches, and the lockfile records their SHA-256 fingerprints. Install with the pinned pnpm 12.14.0 and `pnpm install --frozen-lockfile`; preserve the workspace configuration and patch directory in build contexts.

| Package | Advisory | Patch SHA-256 |
| --- | --- | --- |
| node-forge 1.4.0 | [GHSA-86w9-cpqp-85rv / CVE-2026-85393](https://github.com/advisories/GHSA-86w9-cpqp-85rv) | `b6876574a5de19521a766856b293aeae2689848c5d8dab03f93040be0a64b8b4` |
| braces 3.0.3 | [GHSA-vfj7-8cjw-p6xm / CVE-2026-93687](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm) | `523afce3b412ced1d821c7ff74be8af5b233ec7f7964bcfcf3d45cfa55539365` |

## node-forge

The runtime change is the nested `DigestAlgorithm` element-count check proposed in [upstream PR 1152](https://github.com/digitalbazaar/forge/pull/1152), pinned to commit [`ceba34402e329f0365134f23fe19898756527d65`](https://github.com/digitalbazaar/forge/commit/ceba34402e329f0365134f23fe19898756527d65). It accepts exactly an algorithm OID and its optional NULL parameters, rejecting unconsumed nested children. This backport changes the CommonJS implementation used by the application dependency graph. It does not regenerate upstream standalone browser bundles under `dist`; do not import those prebuilt bundles as a substitute for the patched module.

The regression suite includes the upstream public-key/signature vector and generated e=3 RSA signatures containing injected nested OCTET STRINGs, duplicate NULLs, and nested sequences. Verification uses the normal public API with its default padding checks. Valid signatures with and without the optional SHA-256 NULL parameters continue to verify. The upstream vector and malformed signatures were accepted by the unpatched package and are rejected after patching.

## braces

This is a local bounded-depth mitigation based on the [advisory](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm) and [upstream issue 70](https://github.com/micromatch/braces/issues/70), rather than an upstream release or claimed upstream backport. The parser rejects a 128th nested brace/parenthesis block. Before compile, expand, or stringify can enter recursive walkers, an iterative check rejects ASTs deeper than 128 child edges. This also rejects child cycles, while allowing the normal parser's parent references. The limit is deliberately hard and cannot be disabled by `rangeLimit` or `maxLength` options.

The checks reside in the implementation modules as well as the parser, so both public string/AST entrypoints and direct walker imports are covered. Escaped braces, quoted braces, ordinary alternatives/ranges, and reasonable nesting retain their existing behavior. Extremely deep patterns that previously succeeded on some engines now throw `SyntaxError`; callers accepting untrusted patterns must handle validation errors. Existing expansion-size limits remain separate and unchanged.

## Verification and removal

Run `pnpm run test:security`, `pnpm run lint`, and `pnpm run build`. The attack suite deliberately fails against the original packages. It includes strings under braces' existing 10,000-character limit, supplied ASTs, child cycles, and compatibility cases. No external services or production keys are used.

Version-only `pnpm audit` continues to report these two advisories. Report them as locally mitigated with checked patch fingerprints and passing attack tests, not as fixed upstream or silently ignored. When maintainers publish fixed releases, upgrade each dependency, remove only its corresponding patch entry/file, regenerate the lockfile, and rerun the same regressions before removing the mitigation.

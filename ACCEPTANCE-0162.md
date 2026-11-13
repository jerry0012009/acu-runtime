# Runtime 0.1.62 Learning Controls And Output Prices

Learning controls sit directly below the always-visible graph, before highlights.
They use the same authenticated website GET/PUT settings endpoint, full four-flag
contract, nullable observer interval and administrator global gate. Merely viewing
the page does not write settings. Changes save immediately, serialize writes,
preserve independent flags/intervals, and retain the last confirmed value on error.
Account switches discard old responses. No model Key is used for these settings.

Prices now show model and output price only, with name/ascending/descending sorting.
Numeric strings and zero are valid; unavailable prices sort last in both directions.
Equal prices use stable model-name ordering. Long-context output tiers remain in
the price cell; input/cache prices and multipliers are no longer displayed. Billing
rules, historical charges and the upstream price payload are unchanged.

Local verification covers Go session/auth/validation/account-switch contracts,
114 JS tests, preference controls across eight theme/viewport cases, persistence,
save failure, global disable, keyboard master switch, interval/reset and output
sorting (including zero/unknown). The explicit cloud Go test performs GET/PUT/GET
with unchanged current values through Runtime's handler and normal website session.
Native desktop, packaged kernel, installer and upgrade checks run in release CI.

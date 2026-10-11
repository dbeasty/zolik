/**
 * The web build's half of localNotify.native.ts: a browser shows its own
 * notifications through the service worker (see TableEvents.tsx), so there is
 * nothing to ask for or schedule here.
 */
export async function askToNotify(): Promise<void> {}

export async function notifyNow(_id: string, _title: string, _body: string, _url: string): Promise<void> {}

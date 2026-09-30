/**
 * Turning this device into a node of the database, once there is somebody to
 * be one for.
 *
 * The order matters and is not obvious. The phone has to be running its own
 * server before it has a node key at all - the key is made on first start and
 * kept beside the database - so enrolling is: start the host, read the key it
 * made, hand the public half to the cloud, and start again in node mode with
 * the credential the cloud gave back. After that the credential is kept and
 * the dance is not repeated.
 *
 * Nothing here runs on the web build or in Expo Go: there is no embedded
 * server there, and a person using those simply talks to the cloud as before.
 */

import * as nearby from '../../modules/zolik-nearby';

/** What the cloud answers with when a device enrols. */
export type Enrolment = { nodeId: string; credential: string };

/** Where the credential is kept, per account: it is that account's device. */
export function credentialKey(userHex: string): string {
  return `zolik_node_credential_${userHex}`;
}

type Storage = {
  getItem(key: string): Promise<string | null>;
  setItem(key: string, value: string): Promise<void>;
  deleteItem(key: string): Promise<void>;
};

/** How the cloud is asked to enrol this device. */
type Enroller = (pubkey: string, kind: string) => Promise<Enrolment>;

/**
 * Makes this device hold the signed-in account's data, enrolling it with the
 * cloud first if it has never been enrolled for this account.
 *
 * Returns false where there is no embedded server to be a node with, which is
 * not a failure: it is the web build, and the app goes on reading everything
 * from the cloud.
 */
export async function startNodeFor(
  userHex: string,
  storage: Storage,
  enrol: Enroller,
  cloudBaseUrl: string,
): Promise<boolean> {
  if (!nearby.nearbyAvailable || !userHex) return false;

  const key = credentialKey(userHex);
  let credential = await storage.getItem(key);
  if (!credential) {
    // The key exists only once the host has run once, so the host comes
    // first and node mode second.
    await nearby.startHost();
    const identity = nearby.nodeIdentity();
    if (!identity?.publicKey) return false;
    const enrolment = await enrol(identity.publicKey, 'phone');
    credential = enrolment.credential;
    await storage.setItem(key, credential);
    // Stopped rather than left running: node mode is the same host with a
    // different sync set, and it is started fresh rather than reconfigured
    // underneath whatever is using it.
    await nearby.stopHost();
  }
  await nearby.startNode(credential, userHex, cloudBaseUrl);
  return true;
}

/**
 * Stops this device holding an account's data, for a sign-out.
 *
 * The credential is forgotten with it: it names this device as that person's,
 * and somebody who signs out on a shared phone should not leave it enrolled
 * as theirs.
 */
export async function stopNodeFor(userHex: string, storage: Storage): Promise<void> {
  await nearby.stopHost();
  if (userHex) await storage.deleteItem(credentialKey(userHex));
}

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
 * Who is using the one embedded server.
 *
 * The phone has a single server, and two things want it: node mode, which
 * holds the signed-in account's data, and a table this phone hosts for the
 * room. Either stopping it for its own reasons used to stop it for the other
 * too — leaving a hosted table ended node mode, and signing out ended a table
 * people were still sitting at. Each now says it is done, and the server stops
 * only when neither is using it.
 */
const users = { node: false, table: false };

/** Whether this phone is hosting a table for the room right now. */
export function hostingTable(): boolean {
  return users.table;
}

/** Whether node mode is running on this phone's server. */
export function runningNode(): boolean {
  return users.node;
}

/**
 * Starts (or joins) the embedded server to host a table. A server already in
 * node mode is the same server, and the table is served by it as it stands.
 */
export async function hostTable(): Promise<nearby.HostInfo> {
  const host = await nearby.startHost();
  users.table = true;
  return host;
}

/**
 * The hosted table is over. The server goes with it unless node mode is
 * still using it.
 */
export async function releaseTable(): Promise<void> {
  // Not only after hostTable: a desktop window can hold a host seat the app
  // opened (sitFromApp), and leaving it stopped the server before too.
  users.table = false;
  if (!users.node) await nearby.stopHost();
}

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
  // Starting node mode restarts a server that was serving nobody, and the
  // enrolment below stops it outright: either would end the table. The
  // caller tries again once the table is over.
  if (users.table && !users.node) return false;

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
  users.node = true;
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
  users.node = false;
  // A table this phone is hosting keeps its server: the people at it did not
  // sign out. It stops when the table does (releaseTable).
  if (!users.table) await nearby.stopHost();
  if (userHex) await storage.deleteItem(credentialKey(userHex));
}

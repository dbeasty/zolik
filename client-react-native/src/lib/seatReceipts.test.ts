import { claimLinkFor, receiptFromHash } from './seatReceipts';

jest.mock('@/src/context/SessionContext', () => ({ storage: {} }));

describe('claim links', () => {
  it('carries the receipt in the fragment, where no server sees it', () => {
    const link = claimLinkFor('a.b-c_d', 'https://jokerless.com/');
    expect(link).toBe('https://jokerless.com/claim#r=a.b-c_d');
    expect(new URL(link).search).toBe('');
  });

  it('reads back what it wrote', () => {
    const receipt = 'eyJhbGciOiJFZERTQSJ9.e30.sig+/=';
    expect(receiptFromHash(new URL(claimLinkFor(receipt, 'https://x.test')).hash)).toBe(receipt);
  });

  it('finds nothing in a fragment without one', () => {
    expect(receiptFromHash('')).toBe('');
    expect(receiptFromHash('#x=1')).toBe('');
  });
});

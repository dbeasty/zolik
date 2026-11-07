import { classifyAgeError, classifyAgeRange, overrideStatus } from './ageRange';

describe('classifyAgeRange', () => {
  it('reads an Apple range of 18 and over as adult', () => {
    expect(classifyAgeRange({ lowerBound: 18, upperBound: null })).toBe('adult');
  });
  it('reads an Apple range below 18 as minor', () => {
    expect(classifyAgeRange({ lowerBound: 13, upperBound: 17 })).toBe('minor');
    expect(classifyAgeRange({ lowerBound: null, upperBound: 17 })).toBe('minor');
  });
  it('reads Google Play statuses', () => {
    expect(classifyAgeRange({ userStatus: 'VERIFIED' })).toBe('adult');
    expect(classifyAgeRange({ userStatus: 'SUPERVISED', lowerBound: 13, upperBound: 15 })).toBe('minor');
    expect(classifyAgeRange({ userStatus: 'SUPERVISED_APPROVAL_PENDING' })).toBe('minor');
    expect(classifyAgeRange({ userStatus: 'UNKNOWN' })).toBe('unknown');
    expect(classifyAgeRange({ userStatus: null })).toBe('unknown');
  });
});

describe('classifyAgeError', () => {
  it('treats a declined request as under 18', () => {
    expect(classifyAgeError({ code: 'ERR_AGE_RANGE_USER_DECLINED' })).toBe('minor');
  });
  it('treats anything else as no signal', () => {
    expect(classifyAgeError({ code: 'ERR_AGE_RANGE_NOT_AVAILABLE' })).toBe('unknown');
    expect(classifyAgeError(new Error('boom'))).toBe('unknown');
    expect(classifyAgeError(null)).toBe('unknown');
  });
});

describe('overrideStatus', () => {
  it('accepts only the three settled answers', () => {
    expect(overrideStatus('minor')).toBe('minor');
    expect(overrideStatus('adult')).toBe('adult');
    expect(overrideStatus('pending')).toBeNull();
    expect(overrideStatus(undefined)).toBeNull();
  });
});

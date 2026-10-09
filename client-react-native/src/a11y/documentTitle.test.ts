import { pageTitle, screenTitle } from '@/src/a11y/documentTitle';
import { invalidWhen, domId } from '@/src/a11y/props';
import { radioProps } from '@/src/a11y/RadioGroup';
import { setLocale } from '@/src/lib/i18n';

afterEach(() => setLocale('en'));

describe('pageTitle', () => {
  it('puts the screen first and the app after it', () => {
    expect(pageTitle('Rules')).toBe('Rules · Jokerless');
  });

  it('is the app alone for the home screen or a screen with no title', () => {
    expect(pageTitle('Jokerless')).toBe('Jokerless');
    expect(pageTitle(undefined)).toBe('Jokerless');
    expect(pageTitle('   ')).toBe('Jokerless');
  });
});

describe('screenTitle', () => {
  it('reads the title a route declares, then its header title', () => {
    expect(screenTitle({ title: 'Settings' })).toBe('Settings');
    expect(screenTitle({ headerTitle: 'Table' })).toBe('Table');
    expect(screenTitle({ title: 'Settings', headerTitle: 'Other' })).toBe('Settings');
  });

  it('ignores a header title that is a component, not words', () => {
    expect(screenTitle({ headerTitle: () => null })).toBeUndefined();
    expect(screenTitle(undefined)).toBeUndefined();
  });
});

describe('form and radio props', () => {
  it('marks a field invalid only while there is an error', () => {
    expect(invalidWhen('', 'err-1')).toEqual({});
    expect(invalidWhen(undefined, 'err-1')).toEqual({});
  });

  it('makes React ids safe to use in an id list', () => {
    expect(domId('field', ':r1a:')).toBe('field-r1a');
  });

  it('gives a radio its checked state in the form react-native-web renders', () => {
    expect(radioProps(true)).toMatchObject({ role: 'radio', 'aria-checked': true });
    expect(radioProps(false, 'Large')).toMatchObject({ 'aria-checked': false, 'aria-label': 'Large' });
  });
});

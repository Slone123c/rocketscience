/** Every page shows errors with String(e); people should read the message, not the JS class name. */
export function installErrorDisplay() {
  Error.prototype.toString = function (this: Error) { return this.message || this.name; };
}
